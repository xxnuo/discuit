package program

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/discuitnet/discuit/config"
	"github.com/discuitnet/discuit/core"
	dbx "github.com/discuitnet/discuit/internal/db"
	"github.com/discuitnet/discuit/internal/httperr"
	"github.com/discuitnet/discuit/internal/images"
	"github.com/discuitnet/discuit/internal/taskrunner"
	"github.com/discuitnet/discuit/internal/uid"
	"github.com/discuitnet/discuit/server"
	"github.com/go-sql-driver/mysql"
	"github.com/gomodule/redigo/redis"
	"gorm.io/gorm"
)

type Program struct {
	conf      *config.Config
	db        *gorm.DB
	imagesDir string
	ctx       context.Context
	tr        *taskrunner.TaskRunner
	server    *server.Server
}

func NewProgram(openDatabase bool) (*Program, error) {
	var (
		err error
		pg  = new(Program)
	)

	pg.ctx = context.Background()

	pg.conf, err = config.Parse("config.yaml") // in the working directory
	if err != nil {
		return nil, fmt.Errorf("error parsing the config file: %w", err)
	}

	// Set the images directory:
	pg.imagesDir = "images" // in the working directory
	if pg.conf.ImagesFolderPath != "" {
		pg.imagesDir = pg.conf.ImagesFolderPath
	}
	pg.imagesDir, err = filepath.Abs(pg.imagesDir)
	if err != nil {
		return nil, fmt.Errorf("error attempting to set the images folder location (%s): %w", pg.imagesDir, err)
	}
	images.SetImagesRootFolder(pg.imagesDir)

	pg.tr = taskrunner.New(pg.ctx)

	if openDatabase {
		if _, err := pg.OpenDatabase(); err != nil {
			return nil, err
		}
	}

	return pg, nil
}

func (pg *Program) startBackgroundTasks(delay time.Duration) {
	if pg.db == nil {
		panic("pg.db is nil")
	}

	pg.tr.New("Purge temp posts", func(ctx context.Context) error {
		return core.PurgePostsFromTempTables(ctx, pg.db)
	}, time.Hour, false)
	pg.tr.New("Delete temp images", func(ctx context.Context) error {
		n, err := core.RemoveTempImages(ctx, pg.db)
		log.Printf("Removed %d temp images\n", n)
		return err
	}, time.Hour, false)
	pg.tr.New("Send welcome notifications", func(ctx context.Context) error {
		if pg.conf.WelcomeCommunity == "" {
			log.Println("Config.WelcomeCommunity is empty; skipping sending welcome notifications.")
			return nil
		}
		n, err := core.SendWelcomeNotifications(ctx, pg.db, pg.conf.WelcomeCommunity, time.Hour*6)
		if n > 0 {
			log.Printf("%d welcome notifications successfully sent\n", n)
		}
		return err
	}, time.Minute, false)
	pg.tr.New("Send announcement notifications", func(ctx context.Context) error {
		t0 := time.Now()
		if err := core.SendAnnouncementNotifications(ctx, pg.db, uid.ID{}); err != nil {
			return err
		}
		took := time.Since(t0)
		if took > time.Millisecond*100 {
			log.Printf("Took %v to send announcement notifications\n", time.Since(t0))
		}
		return nil
	}, time.Second*10, false)
	pg.tr.New("Record basic site analytics", func(ctx context.Context) error {
		return core.RecordBasicSiteStats(ctx, pg.db)
	}, time.Hour, false)
	pg.tr.New("Remove expires IP blocks", func(ctx context.Context) error {
		count, err := pg.server.CancelExpiredIPBlocks(context.Background())
		if err != nil {
			return err
		}
		if count > 0 {
			s := ""
			if count > 1 {
				s = "s"
			}
			log.Printf("Cancelled %d expired IP block%s\n", count, s)
		}
		return nil
	}, time.Second*100, false)

	go func() {
		time.Sleep(delay)
		pg.tr.Start()
	}()
}

func (pg *Program) stopBackgroundTasks(ctx context.Context) {
	if err := pg.tr.Stop(ctx); err != nil {
		if errors.Is(err, ctx.Err()) {
			log.Println("Forcefully exited (some) of the background tasks")
		} else {
			log.Printf("Background tasks stop error: %v\n", err)
		}
	} else {
		log.Println("Gracefully exited all background tasks")
	}
}

func (pg *Program) OpenDatabase() (*gorm.DB, error) {
	if pg.db != nil {
		return pg.db, nil
	}

	var err error
	driver, dsn, err := pg.databaseConfig()
	if err != nil {
		return nil, err
	}
	if pg.db, err = openDatabase(driver, dsn); err != nil {
		return nil, fmt.Errorf("error opening database: %w", err)
	}
	return pg.db, nil
}

func (pg *Program) Serve() error {
	if err := pg.createSentinelUsers(); err != nil {
		return fmt.Errorf("error creating sentinel users: %w", err)
	}

	if pg.conf.IsDevelopment {
		if _, err := pg.createDevAdminUser(); err != nil {
			log.Printf("Error creating dev admin user: %v\n", err)
		}
	}

	if err := core.NewBadgeType(pg.db, "supporter"); err != nil {
		return fmt.Errorf("error creating 'supporter' user badge: %w", err)
	}

	if !config.AddressValid(pg.conf.Addr) {
		return errors.New("address needs to be a valid address of the form 'host:port' (host can be empty)")
	}

	site, err := server.New(pg.db, pg.conf)
	if err != nil {
		return fmt.Errorf("error creating server: %w", err)
	}
	defer site.Close()

	pg.server = site

	var https bool = pg.conf.CertFile != ""

	server := &http.Server{
		Addr: pg.conf.Addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Redirect all www. requests to a non-www. host.
			if withoutWWW, found := strings.CutPrefix(r.Host, "www."); found {
				url := *r.URL
				url.Host = withoutWWW
				if https {
					url.Scheme = "https"
				} else {
					url.Scheme = "http"
				}
				http.Redirect(w, r, url.String(), http.StatusMovedPermanently)
				return
			}
			site.ServeHTTP(w, r)
		}),
	}

	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill) // interrupt context
	defer stop()

	var redirectServer *http.Server

	// Optionally start a server to redirect traffic from HTTP to HTTPS.
	if https && pg.conf.Addr[strings.Index(pg.conf.Addr, ":"):] == ":443" {
		redirectServer = &http.Server{
			Addr: ":80",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				url := *r.URL
				url.Scheme = "https"
				url.Host = r.Host
				http.Redirect(w, r, url.String(), http.StatusMovedPermanently)
			}),
		}
		go func() {
			log.Println("Starting redirect server (HTTP -> HTTPS) on " + redirectServer.Addr)
			if err := redirectServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Printf("ListenAndServe (redirect) error: %v\n", err)
			}
		}()
	}

	// Start the server.
	go func() {
		log.Println("Starting server on " + pg.conf.Addr)
		if pg.conf.IsDevelopment {
			log.Println("\033[93;103mStarting server in development mode\033[0m")
		} else {
			if pg.conf.UseHTTPCookies {
				log.Println("\033[93;103mWarning: using unsecure HTTP cookies in production\033[0m")
			}
		}
		if https {
			if err := server.ListenAndServeTLS(pg.conf.CertFile, pg.conf.KeyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Printf("ListenAndServeTLS (main) error: %v\n", err)
			}
		} else {
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Printf("ListenAndServe (main) error: %v\n", err)
			}
		}
	}()

	pg.startBackgroundTasks(time.Second)

	// Wait for interrupt signal.
	<-stopCtx.Done()

	log.Println("Shutting down HTTP server...")

	// Send another interrupt to exit immediately.
	stopCtx, stop = signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer stop()

	if err := server.Shutdown(stopCtx); err != nil {
		if errors.Is(err, stopCtx.Err()) {
			log.Println("Forcefully exited HTTP server")
		} else {
			log.Printf("HTTP server shutdown error: %v\n", err)
		}
	} else {
		log.Println("HTTP Server exited gracefully")
	}
	if redirectServer != nil {
		if err := redirectServer.Shutdown(stopCtx); err != nil {
			if !errors.Is(err, stopCtx.Err()) {
				log.Printf("Redirect server (HTTP -> HTTP) shutdown error: %v\n", err)
			}
		}
	}

	pg.stopBackgroundTasks(stopCtx)
	return nil
}

func (pg *Program) Config() *config.Config {
	var c = new(config.Config)
	*c = *pg.conf
	return c
}

func (pg *Program) Close() error {
	if pg.db != nil {
		return dbx.Close(pg.db)
	}
	return nil
}

func openDatabase(driver, dsn string) (*gorm.DB, error) {
	db, err := dbx.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	if err = dbx.Ping(db); err != nil {
		return nil, fmt.Errorf("failed to ping the database: %w", err)
	}
	return db, nil
}

// createSentinelUsers creates the ghost user only if migrations have been run. If
// migrations have not yet been run, the function exists silently without
// returning an error
func (pg *Program) createSentinelUsers() error {
	if _, err := pg.MigrationsStatus(); err != nil {
		if err == ErrMigrationsTableNotFound || dbx.IsNotFound(err) {
			log.Println("Skipping creating ghost user, as migrations are not yet run.")
			return nil
		}
		log.Printf("Error creating sentinel users: %v\n", err)
		return err
	}

	// Create the ghost user:
	created, err := core.CreateGhostUser(pg.db)
	if err != nil {
		log.Printf("Error creating the ghost user: %v\n", err)
		return err
	}
	if created {
		log.Println("User @ghost succesfully created.")
	}

	// Create the nobody user:
	created, err = core.CreateNobodyUser(pg.db)
	if err != nil {
		log.Printf("Error creating the nobody user: %v\n", err)
		return err
	}
	if created {
		log.Println("User @nobody succesfully created.")
	}

	return nil
}

// MysqlDSN returns a DSN that could be used to connect to a MySQL database. You
// may want to append mysql:// to the beginning of the returned string.
func MysqlDSN(addr, user, password, dbName string) string {
	cfg := mysql.NewConfig()
	cfg.Net = "tcp"
	cfg.Addr = addr
	cfg.User = user
	cfg.Passwd = password
	cfg.DBName = dbName
	cfg.ParseTime = true
	return cfg.FormatDSN()
}

func (pg *Program) HardReset() error {
	if pg.db != nil {
		if err := dbx.Close(pg.db); err != nil {
			return err
		}
		pg.db = nil
	}

	driver, dsn, err := pg.databaseConfig()
	if err != nil {
		return err
	}
	if err := dbx.HardReset(driver, dsn); err != nil {
		return err
	}
	log.Println("Database destroyed and recreated")

	if _, err := pg.OpenDatabase(); err != nil {
		return err
	}

	if err = pg.Migrate(true, 0); err != nil {
		return err
	}

	conn, err := redis.Dial("tcp", pg.conf.RedisAddress)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err = conn.Do("flushall"); err != nil {
		return err
	}

	log.Println("Redis flushed")
	log.Println("Reset complete")

	return nil
}

func (pg *Program) databaseConfig() (string, string, error) {
	driver, err := dbx.NormalizeDriver(pg.conf.DBDriver)
	if err != nil {
		return "", "", err
	}

	if dsn := strings.TrimSpace(pg.conf.DBDSN); dsn != "" {
		return string(driver), dsn, nil
	}

	switch driver {
	case dbx.DriverMariaDB:
		if pg.conf.DBName == "" {
			return "", "", errors.New("no database selected")
		}
		return string(driver), MysqlDSN(pg.conf.DBAddr, pg.conf.DBUser, pg.conf.DBPassword, pg.conf.DBName), nil
	case dbx.DriverSQLite:
		name := strings.TrimSpace(pg.conf.DBName)
		if name == "" {
			name = "discuit.sqlite3"
		}
		return string(driver), name, nil
	default:
		return "", "", errors.New("dbDSN is required for PostgreSQL")
	}
}

func (pg *Program) NewBadgeType(name string) error {
	if err := core.NewBadgeType(pg.db, name); err != nil {
		return fmt.Errorf("failed to create a new badge: %w", err)
	}
	return nil
}

func (pg *Program) MakeUserMod(community, user string, isMod bool) error {
	thecommunity, err := core.GetCommunityByName(pg.ctx, pg.db, community, nil)
	if err != nil {
		return err
	}

	theuser, err := core.GetUserByUsername(pg.ctx, pg.db, user, nil)
	if err != nil {
		return err
	}

	if err = core.MakeUserModCLI(pg.ctx, pg.db, thecommunity, theuser.ID, isMod); err != nil {
		return err
	}

	return nil
}

func (pg *Program) ChangeUserPassword(user, password string) error {
	theuser, err := core.GetUserByUsername(pg.ctx, pg.db, user, nil)
	if err != nil {
		return err
	}
	if theuser.Deleted {
		return fmt.Errorf("cannot change deleted user's password")
	}

	if err = pg.setUserPassword(theuser.ID, password); err != nil {
		return err
	}

	log.Println("Password changed successfully")
	return nil
}

func (pg *Program) FixPostHotScores() error {
	return core.UpdateAllPostsHotness(pg.ctx, pg.db)
}

func (pg *Program) DeleteUser(user string, purge bool) error {
	site, err := server.New(pg.db, pg.conf)
	if err != nil {
		return fmt.Errorf("error creating server: %w", err)
	}
	defer site.Close()

	theuser, err := core.GetUserByUsername(pg.ctx, pg.db, user, nil)
	if err != nil {
		return err
	}

	if theuser.IsGhost() {
		theuser.UnsetToGhost()
	}

	if err := site.LogoutAllSessionsOfUser(theuser); err != nil {
		return err
	}

	if purge {
		nobody, err := core.GetUserByUsername(pg.ctx, pg.db, core.NobodyUserUsername, nil)
		if err != nil {
			return err
		}
		if err := theuser.DeleteContent(pg.ctx, pg.db, 0, nobody.ID); err != nil {
			return err
		}
		log.Println("User content all purged")
	}

	if err := theuser.Delete(pg.ctx, pg.db); err != nil {
		if err == core.ErrUserDeleted {
			log.Printf("%s has been deleted previously\n", theuser.Username)
			return nil
		}
		return err
	}

	log.Printf("%s successfully deleted\n", theuser.Username)

	return nil
}

func (pg *Program) DeleteUnusedCommunities(days uint, dryRun bool) error {
	names, err := core.DeleteUnusedCommunities(pg.ctx, pg.db, days, dryRun)
	if err != nil {
		log.Printf("Failed to delete unused communities older than %d days: %v", days, err)
		return err
	}
	slices.Sort(names)

	if len(names) == 0 {
		log.Println("There are no unused communities to delete.")
	} else {
		var b strings.Builder
		for i, name := range names {
			if i != 0 {
				b.WriteString(", ")
			}
			b.WriteString(name)
		}
		b.WriteString(".")
		log.Printf("Successfully deleted %d unused communities with 0 posts in them: %s", len(names), b.String())
	}

	return nil
}

func (pg *Program) MakeUserAdmin(username string, isAdmin bool) error {
	user, err := core.MakeAdmin(pg.ctx, pg.db, username, isAdmin)
	if err != nil {
		return fmt.Errorf("failed to make %s an admin: %w", username, err)
	}
	if isAdmin {
		log.Printf("User %s is now an admin\n", user.Username)
	} else {
		log.Printf("User %s is no longer an admin", user.Username)
	}
	return nil
}

func (pg *Program) AddAllUsersToCommunity(community string) error {
	if err := core.AddAllUsersToCommunity(pg.ctx, pg.db, community); err != nil {
		return fmt.Errorf("failed to add all users to %s: %w", community, err)
	}
	log.Printf("All users added to %s\n", community)
	return nil
}

func (pg *Program) setUserPassword(userID uid.ID, password string) error {
	pass, err := core.HashPassword([]byte(password))
	if err != nil {
		return err
	}
	return pg.db.WithContext(pg.ctx).
		Model(&dbx.User{}).
		Where("id = ?", userID).
		Update("password", string(pass)).
		Error
}

func (pg *Program) createDevAdminUser() (*core.User, error) {
	const username = "test"
	const password = "test"

	user, err := core.GetUserByUsername(pg.ctx, pg.db, username, nil)
	if err != nil {
		if !httperr.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get dev admin user: %w", err)
		}
		user, err = core.RegisterUser(pg.ctx, pg.db, username, "", password, "")
		if err != nil {
			return nil, fmt.Errorf("failed to register dev admin user: %w", err)
		}
		log.Printf("Dev admin user '%s' created (password: '%s')\n", username, password)
	}
	if user.Deleted {
		return nil, fmt.Errorf("dev admin user is deleted")
	}
	if err := pg.setUserPassword(user.ID, password); err != nil {
		return nil, fmt.Errorf("failed to set dev admin user password: %w", err)
	}
	if !user.Admin {
		if _, err := core.MakeAdmin(pg.ctx, pg.db, user.Username, true); err != nil {
			return nil, fmt.Errorf("failed to make dev admin user: %w", err)
		}
		user.Admin = true
	}
	return user, nil
}

func (pg *Program) Seed() error {
	if err := pg.createSentinelUsers(); err != nil {
		return fmt.Errorf("error creating sentinel users: %w", err)
	}
	adminUser, err := pg.createDevAdminUser()
	if err != nil {
		return err
	}

	communityDefs := []struct {
		name  string
		about string
	}{
		{"General", "General discussion about anything and everything."},
		{"Technology", "News and discussions about technology, gadgets, and software."},
		{"Gaming", "Discuss your favorite games, share tips, and find teammates."},
		{"Science", "Scientific discoveries, research, and discussions."},
		{"Music", "Share and discuss music of all genres."},
		{"Movies", "Movie reviews, recommendations, and discussions."},
		{"Sports", "All things sports - scores, highlights, and discussions."},
		{"Food", "Recipes, restaurant reviews, and food photography."},
		{"Travel", "Travel stories, tips, and destination recommendations."},
		{"Art", "Share and appreciate art in all its forms."},
	}

	var communityIDs []struct {
		id   uid.ID
		name string
	}
	for _, cd := range communityDefs {
		comm, err := core.CreateCommunity(pg.ctx, pg.db, adminUser.ID, 0, 999, cd.name, cd.about)
		if err != nil {
			log.Printf("Community '%s' may already exist, skipping: %v\n", cd.name, err)
			existing, getErr := core.GetCommunityByName(pg.ctx, pg.db, cd.name, nil)
			if getErr != nil {
				continue
			}
			communityIDs = append(communityIDs, struct {
				id   uid.ID
				name string
			}{existing.ID, existing.Name})
			continue
		}
		communityIDs = append(communityIDs, struct {
			id   uid.ID
			name string
		}{comm.ID, comm.Name})
		log.Printf("Created community: %s\n", cd.name)
	}

	userDefs := []string{"alice", "bob", "charlie", "diana", "eve", "frank", "grace", "henry"}
	var userIDs []uid.ID
	userIDs = append(userIDs, adminUser.ID)

	for _, username := range userDefs {
		user, err := core.RegisterUser(pg.ctx, pg.db, username, "", username, "")
		if err != nil {
			existing, getErr := core.GetUserByUsername(pg.ctx, pg.db, username, nil)
			if getErr != nil {
				log.Printf("User '%s' creation failed, skipping: %v\n", username, err)
				continue
			}
			userIDs = append(userIDs, existing.ID)
			continue
		}
		userIDs = append(userIDs, user.ID)
		log.Printf("Created user: %s\n", username)
	}

	for _, ci := range communityIDs {
		for _, uid := range userIDs {
			comm, err := core.GetCommunityByID(pg.ctx, pg.db, ci.id, nil)
			if err == nil {
				comm.Join(pg.ctx, pg.db, uid)
			}
		}
	}

	postTexts := []struct {
		title string
		body  string
	}{
		{"Welcome to Discuit!", "This is a test post to help you get started with the platform. Feel free to explore!"},
		{"What are you working on today?", "Share what projects or tasks you're focusing on. Let's motivate each other!"},
		{"Best resources for learning programming", "I've been collecting great resources for learning to code. What are your favorites?"},
		{"Weekend plans thread", "What's everyone up to this weekend? Share your plans!"},
		{"Thoughts on the future of AI", "Artificial intelligence is advancing rapidly. What do you think the next big breakthrough will be?"},
		{"Favorite productivity tools", "What tools do you use to stay productive? I'm always looking for new recommendations."},
		{"Book recommendations", "Looking for something good to read. What books have you enjoyed recently?"},
		{"Daily discussion thread", "Use this thread to chat about anything. No topic is off limits!"},
		{"Tips for remote work", "Working from home can be challenging. What tips do you have for staying focused?"},
		{"Share your hobby projects", "What interesting hobby projects are you working on? I'd love to see what people are creating."},
		{"Morning coffee chat", "Just having my morning coffee. What's on your mind today?"},
		{"Interesting articles I found this week", "Here are some articles I found interesting this week. Feel free to share yours too!"},
		{"What games are you playing?", "Currently playing through some indie games. What have you been enjoying lately?"},
		{"Photography tips for beginners", "I just got into photography. Any tips for a complete beginner?"},
		{"Cooking challenge: 5 ingredients or less", "Can you make something delicious with just 5 ingredients? Share your recipes!"},
		{"Favorite podcasts right now", "I listen to podcasts during my commute. What are you listening to?"},
		{"Unpopular opinions thread", "Share your unpopular opinions here. Keep it civil!"},
		{"Home office setup show-off", "Show us your home office setup! I'm looking for inspiration."},
		{"Learning a new language", "I've decided to learn Japanese. Anyone else learning a new language? Tips welcome!"},
		{"Best movies of this year", "What movies have impressed you the most this year?"},
	}

	commentTexts := []string{
		"Great post! Thanks for sharing.",
		"I completely agree with this.",
		"Interesting perspective, I hadn't thought about it that way.",
		"Can you elaborate on that point?",
		"This is exactly what I was looking for!",
		"I have a different view on this, but I respect your opinion.",
		"Thanks for the recommendation!",
		"I've had a similar experience.",
		"Well said!",
		"This deserves more attention.",
		"Adding to this - there are also some good alternatives worth considering.",
		"I tried this and it worked perfectly!",
		"Bookmarking this for later.",
		"Has anyone else noticed this trend?",
		"Great discussion everyone!",
	}

	postCount := 0
	commentCount := 0
	for i, pt := range postTexts {
		if len(communityIDs) == 0 || len(userIDs) == 0 {
			break
		}
		ci := communityIDs[i%len(communityIDs)]
		authorID := userIDs[i%len(userIDs)]

		post, err := core.CreateTextPost(pg.ctx, pg.db, authorID, ci.id, pt.title, pt.body)
		if err != nil {
			log.Printf("Failed to create post '%s': %v\n", pt.title, err)
			continue
		}
		postCount++

		numComments := 2 + (i % 4)
		for j := 0; j < numComments && j < len(commentTexts); j++ {
			commentAuthor := userIDs[(i+j+1)%len(userIDs)]
			_, err := post.AddComment(pg.ctx, pg.db, commentAuthor, core.UserGroupNormal, nil, commentTexts[(i+j)%len(commentTexts)])
			if err != nil {
				log.Printf("Failed to create comment on post '%s': %v\n", pt.title, err)
				continue
			}
			commentCount++
		}
	}

	log.Printf("Seed complete: %d communities, %d users, %d posts, %d comments\n",
		len(communityIDs), len(userIDs), postCount, commentCount)
	return nil
}
