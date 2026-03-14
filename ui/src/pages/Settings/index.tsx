import { useEffect, useState } from 'react';
import { Helmet } from 'react-helmet-async';
import { useTranslation } from 'react-i18next';
import { useDispatch, useSelector } from 'react-redux';
import { Link } from 'react-router-dom';
import {
  getNotificationsPermissions,
  shouldAskForNotificationsPermissions,
} from '../../PushNotifications';
import CommunityProPic from '../../components/CommunityProPic';
import Dropdown from '../../components/Dropdown';
import { FormField, FormSection } from '../../components/Form';
import ImageEditModal from '../../components/ImageEditModal';
import Input, { Checkbox } from '../../components/Input';
import CommunityLink from '../../components/PostCard/CommunityLink';
import { isDeviceStandalone, mfetchjson, selectImageCopyURL, validEmail } from '../../helper';
import { useIsChanged } from '../../hooks';
import { Mute, Mutes, MuteType } from '../../serverTypes';
import {
  MainState,
  mutesAdded,
  settingsChanged,
  snackAlert,
  snackAlertError,
  topNavbarAutohideDisabledChanged,
  unmuteCommunity,
  unmuteUser,
  userLoggedIn,
} from '../../slices/mainSlice';
import { RootState } from '../../store';
import ChangePassword from './ChangePassword';
import DeleteAccount from './DeleteAccount';
import { getDevicePreference, setDevicePreference } from './devicePrefs';

const Settings = () => {
  const { t, i18n } = useTranslation();
  const dispatch = useDispatch();
  const user = (useSelector<RootState>((state) => state.main.user) as MainState['user'])!;
  const loggedIn = user !== null;

  const mutes = useSelector<RootState>((state) => state.main.mutes) as MainState['mutes'];
  const [aboutMe, setAboutMe] = useState(user.aboutMe || '');
  const [email, setEmail] = useState(user.email || '');

  const [notifsSettings, _setNotifsSettings] = useState({
    upvoteNotifs: !user.upvoteNotificationsOff,
    replyNotifs: !user.replyNotificationsOff,
  });
  const setNotifsSettings = (key: string, val: unknown) => {
    _setNotifsSettings((prev) => {
      return {
        ...prev,
        [key]: val,
      };
    });
  };

  const homeFeedOptions = {
    all: t('settings.homeFeedAll'),
    subscriptions: t('settings.homeFeedSubscriptions'),
  };

  const [homeFeed, setHomeFeed] = useState(user.homeFeed);

  const [rememberFeedSort, setRememberFeedSort] = useState(user.rememberFeedSort);
  const [enableEmbeds, setEnableEmbeds] = useState(!user.embedsOff);
  const [showUserProfilePictures, setShowUserProfilePictures] = useState(
    !user.hideUserProfilePictures
  );
  const [requireAltText, setRequireAltText] = useState(user.requireAltText);

  const fontOptions = {
    custom: t('settings.fontCustom'),
    system: t('settings.fontSystem'),
  };

  // Per-device preferences:
  const [font, setFont] = useState(
    (getDevicePreference('font') ?? 'custom') as keyof typeof fontOptions
  );
  const [infiniteScrollingDisabed, setInfinitedScrollingDisabled] = useState(
    getDevicePreference('infinite_scrolling_disabled') === 'true'
  );
  const [topNavbarAutohideDisabled, setTopNavbarAutohideDisabled] = useState(
    getDevicePreference('top_navbar_autohide_disabled') === 'true'
  );

  const [changed, resetChanged] = useIsChanged([
    aboutMe /*, email*/,
    notifsSettings,
    homeFeed,
    rememberFeedSort,
    enableEmbeds,
    email,
    showUserProfilePictures,
    font,
    infiniteScrollingDisabed,
    requireAltText,
    topNavbarAutohideDisabled,
  ]);

  const applicationServerKey = useSelector<RootState>(
    (state) => state.main.vapidPublicKey
  ) as MainState['vapidPublicKey'];
  const [notificationsPermissions, setNotificationsPermissions] = useState<string>(
    window.Notification && Notification.permission
  );
  useEffect(() => {
    let cleanupFunc: () => void,
      cancelled = false;
    const f = async () => {
      if ('permissions' in navigator) {
        const status = await navigator.permissions.query({ name: 'notifications' });
        const listener = () => {
          if (!cancelled) {
            setNotificationsPermissions(status.state);
          }
        };
        status.addEventListener('change', listener);
        cleanupFunc = () => status.removeEventListener('change', listener);
      }
    };
    f();
    return () => {
      cancelled = true;
      if (cleanupFunc) cleanupFunc();
    };
  }, []);
  const [canEnableWebPushNotifications, setCanEnableWebPushNotifications] = useState(
    shouldAskForNotificationsPermissions(loggedIn, applicationServerKey, false)
  );
  useEffect(() => {
    setCanEnableWebPushNotifications(
      shouldAskForNotificationsPermissions(loggedIn, applicationServerKey, false)
    );
  }, [notificationsPermissions]);

  const handleEnablePushNotifications = async () => {
    await getNotificationsPermissions(loggedIn, applicationServerKey!);
  };

  const deviceStandalone = isDeviceStandalone();

  const handleSave = async () => {
    if (email !== '' && !validEmail(email)) {
      dispatch(snackAlert(t('settings.validEmailRequired')));
      return;
    }
    // Save device preferences first:
    setDevicePreference('font', font);
    setDevicePreference('infinite_scrolling_disabled', infiniteScrollingDisabed ? 'true' : 'false');
    if (deviceStandalone) {
      setDevicePreference(
        'top_navbar_autohide_disabled',
        topNavbarAutohideDisabled ? 'true' : 'false'
      );
      dispatch(topNavbarAutohideDisabledChanged(topNavbarAutohideDisabled));
    }
    try {
      const ruser = await mfetchjson(`/api/_settings?action=updateProfile`, {
        method: 'POST',
        body: JSON.stringify({
          aboutMe,
          upvoteNotificationsOff: !notifsSettings.upvoteNotifs,
          replyNotificationsOff: !notifsSettings.replyNotifs,
          homeFeed,
          rememberFeedSort,
          embedsOff: !enableEmbeds,
          email,
          hideUserProfilePictures: !showUserProfilePictures,
          requireAltText,
        }),
      });
      dispatch(userLoggedIn(ruser));
      dispatch(snackAlert(t('settings.settingsSaved'), 'settings_saved'));
      resetChanged();
      dispatch(settingsChanged());
    } catch (error) {
      dispatch(snackAlertError(error));
    }
  };

  const proPicAPIEndpoint = `/api/users/${user.username}/pro_pic`;
  const [profilePicModalOpen, setProfilePicModalOpen] = useState(false);
  const [isUploadingPic, setIsUploadingPic] = useState(false);
  const [isDeletingPic, setIsDeletingPic] = useState(false);

  const handleUploadProfilePic = async (file: File) => {
    if (isUploadingPic) return;

    try {
      setIsUploadingPic(true);
      const formData = new FormData();
      formData.append('image', file);

      const ruser = await mfetchjson(proPicAPIEndpoint, {
        method: 'POST',
        body: formData,
      });

      dispatch(userLoggedIn(ruser));
    } catch (error) {
      dispatch(snackAlertError(error));
    } finally {
      setIsUploadingPic(false);
    }
  };

  const handleDeleteProfilePic = async () => {
    if (isDeletingPic) return;

    try {
      setIsDeletingPic(true);
      const ruser = await mfetchjson(proPicAPIEndpoint, {
        method: 'DELETE',
      });

      dispatch(userLoggedIn(ruser));
    } catch (error) {
      dispatch(snackAlertError(error));
    } finally {
      setIsDeletingPic(false);
    }
  };

  const handleSaveProfilePicAlt = async (altText: string) => {
    try {
      if (!user.proPic) return dispatch(snackAlert(t('settings.noProfilePicture')));
      const proPicId = user.proPic.id;
      await mfetchjson(`/api/images/${proPicId}`, {
        method: 'PUT',
        body: JSON.stringify({ altText }),
      });

      user.proPic.altText = altText;
      dispatch(snackAlert(t('settings.altTextSaved')));
      setProfilePicModalOpen(false);
    } catch (error) {
      dispatch(snackAlertError(error));
    }
  };

  const handleUnmute = async (mute: Mute) => {
    // try {
    //   await mfetchjson(`/api/mutes/${mute.id}`, {
    //     method: 'DELETE',
    //   });
    //   setMutes((mutes) => {
    //     let array, fieldName;
    //     if (mute.type === 'community') {
    //       array = mutes.communityMutes;
    //       fieldName = 'communityMutes';
    //     } else {
    //       array = mutes.userMutes;
    //       fieldName = 'userMutes';
    //     }
    //     array = array.filter((m) => m.id !== mute.id);
    //     return {
    //       ...mutes,
    //       [fieldName]: array,
    //     };
    //   });
    // } catch (error) {
    //   dispatch(snackAlertError(error));
    // }
    if (mute.type === 'community') {
      const community = mute.mutedCommunity!;
      dispatch(unmuteCommunity(community.id, community.name));
    } else {
      const user = mute.mutedUser!;
      dispatch(unmuteUser(user.id, user.username));
    }
  };

  const handleUnmuteAll = async (type: MuteType) => {
    try {
      await mfetchjson(`/api/mutes?type=${type || ''}`, {
        method: 'DELETE',
      });
      const newMutes: Mutes = {
        ...mutes,
      };
      if (type === 'user') {
        newMutes.userMutes = [];
      } else if (type === 'community') {
        newMutes.communityMutes = [];
      }
      dispatch(mutesAdded(newMutes));
    } catch (error) {
      dispatch(snackAlertError(error));
    }
  };

  const renderMute = (mute: Mute) => {
    if (mute.type === 'community') {
      const community = mute.mutedCommunity!;
      return (
        <div className="mute-list-item">
          <CommunityLink name={community.name} proPic={community.proPic} />
          <button onClick={() => handleUnmute(mute)}>{t('common.unmute')}</button>
        </div>
      );
    }
    if (mute.type === 'user') {
      const user = mute.mutedUser!;
      return (
        <div>
          <Link to={`/@${user.username}`}>@{user.username}</Link>
          <button onClick={() => handleUnmute(mute)}>{t('common.unmute')}</button>
        </div>
      );
    }
    return 'Unkonwn muting type.';
  };

  const communityMutes = mutes.communityMutes || [];
  const userMutes = mutes.userMutes || [];

  return (
    <div className="page-content wrap page-settings">
      <Helmet>
        <title>{t('settings.title')}</title>
      </Helmet>
      <div className="form account-settings card">
        <h1>{t('settings.accountSettings')}</h1>
        <FormSection>
          <FormSection>
            <div className="settings-propic">
              <CommunityProPic name={user.username} proPic={user.proPic} size="standard" />
              <button onClick={() => setProfilePicModalOpen(true)}>{t('settings.editProfilePicture')}</button>
            </div>
          </FormSection>
          <FormField label={t('common.username')} description={t('settings.usernameCannotChange')}>
            <Input value={user.username || ''} disabled />
          </FormField>
          <FormField label={t('common.email')}>
            <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
          </FormField>
          <FormField label={t('settings.aboutMe')}>
            <textarea
              rows={5}
              placeholder={t('settings.aboutMePlaceholder')}
              value={aboutMe}
              onChange={(e) => setAboutMe(e.target.value)}
            />
          </FormField>
          <FormField>
            <ChangePassword />
          </FormField>
          <FormField>
            <DeleteAccount user={user} />
          </FormField>
        </FormSection>
        <FormSection heading={t('settings.preferences')}>
          <FormField className="is-preference" label={t('settings.homeFeed')}>
            <Dropdown
              aligned="right"
              target={<button className="select-bar-dp-target">{homeFeedOptions[homeFeed]}</button>}
            >
              <div className="dropdown-list">
                {Object.keys(homeFeedOptions)
                  .filter((key) => key != homeFeed)
                  .map((_key) => {
                    const key = _key as keyof typeof homeFeedOptions;
                    return (
                      <div key={key} className="dropdown-item" onClick={() => setHomeFeed(key)}>
                        {homeFeedOptions[key]}
                      </div>
                    );
                  })}
              </div>
            </Dropdown>
          </FormField>
          <FormField className="is-preference is-switch">
            <Checkbox
              variant="switch"
              label={t('settings.rememberFeedSort')}
              checked={rememberFeedSort}
              onChange={(e) => setRememberFeedSort(e.target.checked)}
            />
          </FormField>
          <FormField className="is-preference is-switch">
            <Checkbox
              label={t('settings.enableEmbeds')}
              variant="switch"
              checked={enableEmbeds}
              onChange={(e) => setEnableEmbeds(e.target.checked)}
            />
          </FormField>
          <FormField className="is-preference is-switch">
            <Checkbox
              variant="switch"
              label={t('settings.showUserProfilePictures')}
              checked={showUserProfilePictures}
              onChange={(e) => setShowUserProfilePictures(e.target.checked)}
            />
          </FormField>
          <FormField className="is-preference is-switch">
            <Checkbox
              variant="switch"
              label={t('settings.requireAltText')}
              checked={requireAltText}
              onChange={(e) => setRequireAltText(e.target.checked)}
            />
          </FormField>
        </FormSection>
        <FormSection heading={t('settings.devicePreferences')}>
          <FormField className="is-preference" label={t('settings.font')}>
            <Dropdown
              aligned="right"
              target={<button className="select-bar-dp-target">{fontOptions[font]}</button>}
            >
              <div className="dropdown-list">
                {Object.keys(fontOptions).map((_key) => {
                  const key = _key as keyof typeof fontOptions;
                  return (
                    <div key={key} className="dropdown-item" onClick={() => setFont(key)}>
                      {fontOptions[key]}
                    </div>
                  );
                })}
              </div>
            </Dropdown>
          </FormField>
          <FormField className="is-preference is-switch">
            <Checkbox
              variant="switch"
              label={t('settings.enableInfiniteScrolling')}
              checked={!infiniteScrollingDisabed}
              onChange={(e) => setInfinitedScrollingDisabled(!e.target.checked)}
            />
          </FormField>
          {deviceStandalone && (
            <FormField className="is-preference is-switch">
              <Checkbox
                variant="switch"
                label={t('settings.autoHideTopNavbar')}
                checked={!topNavbarAutohideDisabled}
                onChange={(e) => setTopNavbarAutohideDisabled(!e.target.checked)}
              />
            </FormField>
          )}
        </FormSection>
        <FormSection heading={t('settings.language')}>
          <FormField className="is-preference" label={t('settings.language')}>
            <Dropdown
              aligned="right"
              target={<button className="select-bar-dp-target">{i18n.language === 'zh' ? '中文' : 'English'}</button>}
            >
              <div className="dropdown-list">
                {i18n.language !== 'en' && (
                  <div className="dropdown-item" onClick={() => i18n.changeLanguage('en')}>
                    English
                  </div>
                )}
                {i18n.language !== 'zh' && (
                  <div className="dropdown-item" onClick={() => i18n.changeLanguage('zh')}>
                    中文
                  </div>
                )}
              </div>
            </Dropdown>
          </FormField>
        </FormSection>
        <FormSection heading={t('settings.notifications')}>
          <FormField className="is-preference is-switch">
            <Checkbox
              variant="switch"
              label={t('settings.enableUpvoteNotifications')}
              checked={notifsSettings.upvoteNotifs}
              onChange={(e) => setNotifsSettings('upvoteNotifs', e.target.checked)}
            />
          </FormField>
          <FormField className="is-preference is-switch">
            <Checkbox
              variant="switch"
              label={t('settings.enableReplyNotifications')}
              checked={notifsSettings.replyNotifs}
              onChange={(e) => setNotifsSettings('replyNotifs', e.target.checked)}
            />
          </FormField>
          {canEnableWebPushNotifications && (
            <FormField>
              <button onClick={handleEnablePushNotifications} style={{ alignSelf: 'flex-start' }}>
                {t('settings.enablePushNotifications')}
              </button>
            </FormField>
          )}
        </FormSection>
        <FormSection heading={t('settings.mutedCommunities')}>
          <div className="mutes-list">
            {communityMutes.length === 0 && <div>{t('common.none')}</div>}
            {communityMutes.map((mute) => renderMute(mute))}
            {communityMutes.length > 0 && (
              <button
                style={{ alignSelf: 'flex-end' }}
                onClick={() => handleUnmuteAll('community')}
              >
                {t('common.unmuteAll')}
              </button>
            )}
          </div>
        </FormSection>
        <FormSection heading={t('settings.mutedUsers')}>
          <div className="mutes-list">
            {userMutes.length === 0 && <div>{t('common.none')}</div>}
            {userMutes.map((mute) => renderMute(mute))}
            {userMutes.length > 0 && (
              <button style={{ alignSelf: 'flex-end' }} onClick={() => handleUnmuteAll('user')}>
                {t('common.unmuteAll')}
              </button>
            )}
          </div>
        </FormSection>
        <FormField>
          <button
            className="button-main"
            disabled={!changed}
            onClick={handleSave}
            style={{ width: '100%' }}
          >
            {t('common.save')}
          </button>
        </FormField>
      </div>

      <ImageEditModal
        open={profilePicModalOpen}
        onClose={() => setProfilePicModalOpen(false)}
        title={t('settings.editProfilePicture')}
        imageUrl={user.proPic ? selectImageCopyURL('medium', user.proPic) : undefined}
        altText={user.proPic?.altText}
        onUpload={handleUploadProfilePic}
        onDelete={handleDeleteProfilePic}
        onSave={handleSaveProfilePicAlt}
        uploading={isUploadingPic}
        deleting={isDeletingPic}
      />
    </div>
  );
};

export default Settings;
