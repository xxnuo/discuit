# Discuit

This is the codebase that powers [Discuit](https://discuit.org), which is an
open-source community platform, an alternative to Reddit.

Built with:

- [Go](https://go.dev): The backend.
- [React](https://react.dev/): The frontend.
- [MariaDB](https://en.wikipedia.org/wiki/MariaDB) / [SQLite](https://www.sqlite.org/): The main datastore.
- [Redis](https://redis.io/): For transient data.

## Architecture

The project deploys as two separate parts:

- **Backend**: A Go API server running in a Docker container, serving `/api/` and `/images/` routes.
- **Frontend**: A React SPA built with Vite, deployed to a CDN.

## Getting started

### Running locally

To setup a development environment of Discuit on your local computer:

1.  Install Go (1.21 or higher) by following the instructions at
    [go.dev.](https://go.dev/doc/install)
1.  Install Redis, Node.js, and pnpm. On Ubuntu, for instance:

    ```shell
    sudo apt update

    # Install and start Redis
    sudo apt install redis-server
    sudo systemctl start redis.service

    # Install Node.js and pnpm
    sudo apt install nodejs
    npm install -g pnpm
    ```

1.  Discuit uses `libvips` for fast image transformations. Make sure it's
    installed on your computer. On Ubuntu you can install it with:
    `sudo apt install libvips-dev`.
1.  Clone this repository:

    ```shell
    git clone https://github.com/discuitnet/discuit.git && cd discuit
    ```

1.  Start a local Redis container (or use a system Redis):

    ```shell
    make prepare-dev
    ```

1.  Run the dev server (starts both backend and frontend):

    ```shell
    make dev
    ```

After creating an account, you can run `./discuit admin make username` to make
a user an admin of the site.

### Running with Docker (Backend only)

1. **Build the Docker Image**

   ```shell
   docker build -t discuit .
   ```

2. **Run the Docker Container**

   ```shell
   docker run -d --name discuit \
     -v discuit-data:/app/data \
     -v discuit-redis:/var/lib/redis \
     -v discuit-images:/app/images \
     -e DISCUIT_ADDR=":80" \
     -e DISCUIT_DB_DRIVER="sqlite3" \
     -e DISCUIT_DB_DSN="/app/data/discuit.db" \
     -p 8080:80 \
     discuit
   ```

3. **Stopping and Starting**:

   ```shell
   docker stop discuit
   docker start discuit
   ```

## Environment Variables

All configuration can be set via environment variables. If a `config.yaml` file exists in the working directory, it will be loaded first, then environment variables override any values set in YAML.

| Variable | Description | Default |
|---|---|---|
| `DISCUIT_ADDR` | Server listen address (host:port) | `:8080` |
| `DISCUIT_IS_DEVELOPMENT` | Enable development mode | `false` |
| `DISCUIT_USE_HTTP_COOKIES` | Use HTTP (insecure) cookies | `false` |
| `DISCUIT_SITE_NAME` | Site display name | `""` |
| `DISCUIT_SITE_DESCRIPTION` | Site description for meta tags | `""` |
| `DISCUIT_DB_DRIVER` | Database driver (`sqlite3`, `mariadb`, `postgres`) | `sqlite3` |
| `DISCUIT_DB_DSN` | Database DSN | `discuit.db` |
| `DISCUIT_DB_ADDR` | MariaDB address | `""` |
| `DISCUIT_DB_USER` | MariaDB user | `discuit` |
| `DISCUIT_DB_PASSWORD` | MariaDB password | `""` |
| `DISCUIT_DB_NAME` | MariaDB database name | `""` |
| `DISCUIT_SESSION_COOKIE_NAME` | Session cookie name | `SID` |
| `DISCUIT_REDIS_ADDRESS` | Redis address | `:6379` |
| `DISCUIT_HMAC_SECRET` | HMAC secret for signing | `""` |
| `DISCUIT_CSRF_OFF` | Disable CSRF protection | `false` |
| `DISCUIT_NO_LOG_TO_FILE` | Disable file logging | `false` |
| `DISCUIT_PAGINATION_LIMIT` | Default pagination limit | `10` |
| `DISCUIT_PAGINATION_LIMIT_MAX` | Max pagination limit | `50` |
| `DISCUIT_DEFAULT_FEED_SORT` | Default feed sort order | `hot` |
| `DISCUIT_CAPTCHA_SECRET` | Captcha secret (skipped if empty) | `""` |
| `DISCUIT_CAPTCHA_SITEKEY` | Captcha site key | `""` |
| `DISCUIT_CERT_FILE` | TLS certificate file path | `""` |
| `DISCUIT_KEY_FILE` | TLS key file path | `""` |
| `DISCUIT_DISABLE_RATE_LIMITS` | Disable rate limiting | `false` |
| `DISCUIT_MAX_IMAGE_SIZE` | Max image size in bytes | `26214400` |
| `DISCUIT_ADMIN_API_KEY` | Admin API key (disables rate limits) | `""` |
| `DISCUIT_DISABLE_IMAGE_POSTS` | Disable image posts | `false` |
| `DISCUIT_DISABLE_FORUM_CREATION` | Only admins can create communities | `false` |
| `DISCUIT_FORUM_CREATION_REQ_POINTS` | Points required to create a community | **required** |
| `DISCUIT_MAX_FORUMS_PER_USER` | Max communities per user | **required** |
| `DISCUIT_IMAGES_FOLDER_PATH` | Path for image storage | `images` |

### Source code layout

In the root directory are these directories:

- `cli`: Contains the command-line interface.
- `core`: Contains all the core functionality of the backend.
- `internal`: Contains Go packages internal to the project.
- `migrations`: Contains the SQL migration files.
- `server`: Contains the REST API backend.
- `ui` - Contains the React frontend.

## Contributing

Discuit is free and open-source software, and you're welcome to contribute to
its development.

If you're thinking of working on something substantial, however, (like a major
feature) please create an issue, or contact [the
maintainer](https://discuit.org/@previnder), to discuss it before commencing
work.

The documentation of the API can be found at [docs.discuit.org](https://docs.discuit.org).

## License

Copyright (C) 2024 Previnder

This program is free software: you can redistribute it and/or modify it under
the terms of the GNU Affero General Public License as published by the Free
Software Foundation, either version 3 of the License, or (at your option) any
later version.

This program is distributed in the hope that it will be useful, but WITHOUT ANY
WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
PARTICULAR PURPOSE. See the GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License along
with this program. If not, see <https://www.gnu.org/licenses/>.
