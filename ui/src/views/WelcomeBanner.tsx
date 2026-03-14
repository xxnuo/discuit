import clsx from 'clsx';
import { useTranslation, Trans } from 'react-i18next';
import { useDispatch, useSelector } from 'react-redux';
import { Link } from 'react-router-dom';
import { createCommunityModalOpened, MainState, signupModalOpened } from '../slices/mainSlice';
import { RootState } from '../store';

export interface WelcomeBannerProps extends React.HTMLAttributes<HTMLDivElement> {
  hideIfMember?: boolean;
}

const WelcomeBanner = ({
  className,
  children,
  hideIfMember = false,
  ...props
}: WelcomeBannerProps) => {
  const { t } = useTranslation();
  const dispatch = useDispatch();

  const user = useSelector<RootState>((state) => state.main.user) as MainState['user'];
  const loggedIn = user !== null;

  const usersCount = useSelector<RootState>((state) => state.main.noUsers) as MainState['noUsers'];

  if (hideIfMember && loggedIn) {
    return null;
  }

  const canCreateForum = loggedIn && (user.isAdmin || !import.meta.env.VITE_DISABLEFORUMCREATION);

  return (
    <div
      className={clsx(
        'card card-sub card-padding home-welcome',
        !loggedIn && 'is-guest',
        className
      )}
      {...props}
    >
      <div className="home-welcome-text">
        <div className="home-welcome-join">{t('welcome.joinDiscussion')}</div>
        <div className="home-welcome-subtext">
          <Trans i18nKey="welcome.description" values={{ count: usersCount }}>
            Discuit is a place where <span>{'{{count}}'}</span> people get together to find cool stuff and discuss things.
          </Trans>
        </div>
      </div>
      <div className="home-welcome-buttons">
        {loggedIn && (
          <Link to="/new" className={'button' + (loggedIn ? ' button-main' : '')}>
            {t('welcome.createPost')}
          </Link>
        )}
        {canCreateForum && (
          <>
            <button
              onClick={() => dispatch(createCommunityModalOpened())}
              className={'button' + (loggedIn ? ' button-main' : '')}
            >
              {t('welcome.createCommunity')}
            </button>
          </>
        )}
        <>{children}</>
        {!loggedIn && (
          <button onClick={() => dispatch(signupModalOpened())}>{t('welcome.createNewAccount')}</button>
        )}
      </div>
    </div>
  );
};

export default WelcomeBanner;
