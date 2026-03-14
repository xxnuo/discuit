import PropTypes from 'prop-types';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useDispatch, useSelector } from 'react-redux';
import { ButtonClose } from '../../components/Button';
import DashboardPage from '../../components/Dashboard/DashboardPage';
import { FormField } from '../../components/Form';
import Input from '../../components/Input';
import Modal from '../../components/Modal';
import { mfetch } from '../../helper';
import { Community } from '../../serverTypes';
import { MainState, snackAlertError } from '../../slices/mainSlice';
import { RootState } from '../../store';

const Mods = ({ community }: { community: Community }) => {
  const user = (useSelector<RootState>((state) => state.main.user) as MainState['user'])!;
  const dispatch = useDispatch();
  const { t } = useTranslation('modtools');

  const [addModOpen, setAddModOpen] = useState(false);
  const handleAddModClose = () => setAddModOpen(false);
  const [newModName, setNewModName] = useState('');

  const baseURL = `/api/communities/${community.id}/mods`;

  const handleAddMod = async (event?: React.FormEvent | React.MouseEvent) => {
    if (event) {
      event.preventDefault();
    }
    try {
      const res = await mfetch(baseURL, {
        method: 'POST',
        body: JSON.stringify({
          username: newModName,
        }),
      });
      if (res.ok) {
        alert(t('modAdded', { name: newModName, community: community.name }));
        window.location.reload();
      } else if (res.status === 404) {
        alert(t('userNotFound'));
      } else {
        throw new Error(await res.text());
      }
    } catch (error) {
      dispatch(snackAlertError(error));
    }
  };

  const handleRemoveMod = async (username: string) => {
    if (
      !confirm(t('removeModConfirm', { name: username, community: community.name }))
    ) {
      return;
    }
    try {
      const res = await mfetch(`${baseURL}/${username}`, {
        method: 'DELETE',
      });
      if (res.ok) {
        alert(t('modRemoved', { name: username }));
        window.location.reload();
      } else {
        throw new Error(await res.text());
      }
    } catch (error) {
      dispatch(snackAlertError(error));
    }
  };

  const mods = community.mods || [];
  let myPos = 0;
  mods.forEach((mod, index) => {
    if (mod.id === user.id) {
      myPos = index;
    }
  });

  return (
    <DashboardPage
      className="modtools-content modtools-mods"
      title={t('mods')}
      fullWidth
      titleRightContent={
        <button className="button-main" onClick={() => setAddModOpen(true)}>
          {t('addMod')}
        </button>
      }
    >
      <Modal open={addModOpen} onClose={handleAddModClose}>
        <div className="modal-card">
          <div className="modal-card-head">
            <div className="modal-card-title">{t('addNewModerator')}</div>
            <ButtonClose onClick={handleAddModClose} />
          </div>
          <form className="modal-card-content" onSubmit={handleAddMod}>
            <FormField label={t('username')}>
              <Input value={newModName} onChange={(e) => setNewModName(e.target.value)} autoFocus />
            </FormField>
          </form>
          <div className="modal-card-actions">
            <button className="button-main" disabled={newModName === ''} onClick={handleAddMod}>
              {t('addMod')}
            </button>
            <button onClick={handleAddModClose}>{t('common:cancel')}</button>
          </div>
        </div>
      </Modal>
      <div className="modtools-mods-list">
        <div className="table">
          {mods.map((mod, index) => (
            <div className="table-row" key={mod.id}>
              <div className="table-column">{index}</div>
              <div className="table-column">{mod.username}</div>
              <div className="table-column">
                {(myPos <= index || user.isAdmin) && (
                  <button className="button-red" onClick={() => handleRemoveMod(mod.username)}>
                    {t('remove')}
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      </div>
    </DashboardPage>
  );
};

Mods.propTypes = {
  community: PropTypes.object.isRequired,
};

export default Mods;
