import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useDispatch } from 'react-redux';
import { ButtonClose } from '../../components/Button';
import { FormField } from '../../components/Form';
import { InputPassword } from '../../components/Input';
import Modal from '../../components/Modal';
import { APIError, mfetch } from '../../helper';
import { snackAlert, snackAlertError } from '../../slices/mainSlice';

const ChangePassword = () => {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const handleClose = () => setOpen(false);

  const [password, setPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [repeatPassword, setRepeatPassword] = useState('');
  useEffect(() => {
    setPassword('');
    setNewPassword('');
    setRepeatPassword('');
  }, [open]);

  const dispatch = useDispatch();
  const changePassword = async () => {
    if (newPassword !== repeatPassword) {
      alert(t('changePassword.passwordsDoNotMatch'));
      return;
    }
    if (newPassword.length < 8) {
      alert(t('changePassword.passwordTooShort'));
      return;
    }
    try {
      const res = await mfetch('/api/_settings?action=changePassword', {
        method: 'POST',
        body: JSON.stringify({
          password,
          newPassword,
          repeatPassword,
        }),
      });
      if (!res.ok) {
        if (res.status === 401) {
          alert(t('changePassword.incorrectPassword'));
          return;
        }
        throw new APIError(res.status, await res.json());
      } else {
        dispatch(snackAlert(t('changePassword.success')));
        setOpen(false);
      }
    } catch (error) {
      dispatch(snackAlertError(error));
    }
  };

  return (
    <>
      <button onClick={() => setOpen(true)} style={{ alignSelf: 'flex-start' }}>
        {t('changePassword.button')}
      </button>
      <Modal open={open} onClose={handleClose}>
        <div className="modal-card modal-change-password">
          <div className="modal-card-head">
            <div className="modal-card-title">{t('changePassword.title')}</div>
            <ButtonClose onClick={handleClose} />
          </div>
          <div
            className="form modal-card-content"
            onKeyDown={(e) => e.key === 'Enter' && changePassword()}
            role="none"
          >
            <FormField label={t('changePassword.previousPassword')}>
              <InputPassword
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoFocus
              />
            </FormField>
            <FormField label={t('changePassword.newPassword')}>
              <InputPassword value={newPassword} onChange={(e) => setNewPassword(e.target.value)} />
            </FormField>
            <FormField label={t('changePassword.repeatPassword')}>
              <InputPassword
                value={repeatPassword}
                onChange={(e) => setRepeatPassword(e.target.value)}
              />
            </FormField>
          </div>
          <div className="modal-card-actions">
            <button className="button-main" onClick={changePassword}>
              {t('changePassword.title')}
            </button>
            <button onClick={handleClose}>{t('common.cancel')}</button>
          </div>
        </div>
      </Modal>
    </>
  );
};

export default ChangePassword;
