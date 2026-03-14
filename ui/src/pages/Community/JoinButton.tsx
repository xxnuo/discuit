import clsx from 'clsx';
import React from 'react';
import { useTranslation } from 'react-i18next';
import { useDispatch, useSelector } from 'react-redux';
import { mfetchjson } from '../../helper';
import { Community } from '../../serverTypes';
import { communityAdded } from '../../slices/communitiesSlice';
import { loginPromptToggled, MainState, snackAlertError } from '../../slices/mainSlice';
import { RootState } from '../../store';

export interface JoinButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  community: Community;
}

const JoinButton = ({ className, community, ...rest }: JoinButtonProps) => {
  const loggedIn =
    (useSelector<RootState>((state) => state.main.user) as MainState['user']) !== null;
  const dispatch = useDispatch();
  const { t } = useTranslation('community');

  const joined = community ? community.userJoined : false;
  const handleFollow = async () => {
    if (!loggedIn) {
      dispatch(loginPromptToggled());
      return;
    }
    const message = t('leaveModConfirm', { name: community.name });
    if (community.userMod && !confirm(message)) {
      return;
    }
    try {
      const rcomm = await mfetchjson('/api/_joinCommunity', {
        method: 'POST',
        body: JSON.stringify({ communityId: community.id, leave: joined }),
      });
      dispatch(communityAdded(rcomm));
    } catch (error) {
      dispatch(snackAlertError(error));
    }
  };

  return (
    <button onClick={handleFollow} className={clsx(!joined && 'button-main', className)} {...rest}>
      {joined ? t('joined') : t('join')}
    </button>
  );
};

export default JoinButton;
