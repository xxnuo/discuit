import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useDispatch } from 'react-redux';
import { ButtonClose } from '../../components/Button';
import DashboardPage from '../../components/Dashboard/DashboardPage';
import { FormField } from '../../components/Form';
import Input from '../../components/Input';
import Modal from '../../components/Modal';
import { APIError, mfetch, mfetchjson } from '../../helper';
import { useLoading } from '../../hooks';
import { Community, User } from '../../serverTypes';
import { snackAlert, snackAlertError } from '../../slices/mainSlice';

const Banned = ({ community }: { community: Community }) => {
  const dispatch = useDispatch();
  const { t } = useTranslation('modtools');

  const baseURL = `/api/communities/${community.id}`;
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useLoading();
  useEffect(() => {
    (async () => {
      try {
        const banned = await mfetchjson(`${baseURL}/banned`);
        setUsers(banned);
        setLoading('loaded');
      } catch {
        setLoading('error');
      }
    })();
  }, [community.id, baseURL, setLoading]);

  const [modalError, setModalError] = useState('');
  const [username, _setUsername] = useState('');
  const setUsername = (name: string) => {
    if (name === '') setModalError('');
    _setUsername(name);
  };
  const [banModalOpen, setBanModalOpen] = useState(false);
  const handleBanModalClose = () => {
    setBanModalOpen(false);
    setUsername('');
  };
  const handleBanClick = async () => {
    try {
      const res = await mfetch(`${baseURL}/banned`, {
        method: 'POST',
        body: JSON.stringify({
          username,
        }),
      });
      if (!res.ok) {
        if (res.status === 404) {
          setModalError(t('userNotFoundError'));
        } else if (res.status === 409) {
          setModalError(t('alreadyBanned', { name: username }));
        } else if (res.status === 403) {
          dispatch(snackAlert(t('forbidden'), 'forbidden'));
        } else {
          throw new APIError(res.status, await res.json());
        }
      } else {
        dispatch(snackAlert(t('userBanned', { name: username })));
        const user = await res.json();
        setUsers((users) => [...users, user]);
        handleBanModalClose();
      }
    } catch (error) {
      dispatch(snackAlertError(error));
    }
  };

  const handleUnbanClick = async (username: string) => {
    try {
      const user = await mfetchjson(`${baseURL}/banned`, {
        method: 'DELETE',
        body: JSON.stringify({
          username,
        }),
      });
      setUsers((users) => users.filter((u) => u.username !== user.username));
    } catch (error) {
      dispatch(snackAlertError(error));
    }
  };

  if (loading !== 'loaded') {
    return null;
  }

  return (
    <DashboardPage
      className="modtools-banned"
      title={t('bannedCount', { count: users.length })}
      fullWidth
      titleRightContent={
        <button className="button-main" onClick={() => setBanModalOpen(true)}>
          {t('banUser')}
        </button>
      }
    >
      <Modal open={banModalOpen} onClose={handleBanModalClose}>
        <div className="modal-card">
          <div className="modal-card-head">
            <div className="modal-card-title">{t('banUser')}</div>
            <ButtonClose onClick={handleBanModalClose} />
          </div>
          <form
            className="modal-card-content"
            onSubmit={(e) => {
              e.preventDefault();
              handleBanClick();
            }}
          >
            <FormField label={t('username')} error={modalError}>
              <Input value={username} onChange={(e) => setUsername(e.target.value)} autoFocus />
            </FormField>
          </form>
          <div className="modal-card-actions">
            <button className="button-main" disabled={username === ''} onClick={handleBanClick}>
              {t('ban')}
            </button>
            <button onClick={handleBanModalClose}>{t('common:cancel')}</button>
          </div>
        </div>
      </Modal>
      <div className="modtools-banned-users">
        <div className="table">
          {users.map((user) => (
            <div key={user.id} className="table-row">
              <div className="table-column">@{user.username}</div>
              <div className="table-column"></div>
              <div className="table-column">
                <button onClick={() => handleUnbanClick(user.username)}>{t('unban')}</button>
              </div>
            </div>
          ))}
        </div>
      </div>
    </DashboardPage>
  );
};

export default Banned;
