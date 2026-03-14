import { useTranslation } from 'react-i18next';
import { useDispatch, useSelector } from 'react-redux';
import { loginModalOpened, loginPromptToggled, signupModalOpened } from '../slices/mainSlice';
import { RootState } from '../store';
import { ButtonClose } from './Button';
import Modal from './Modal';

const LoginPrompt = () => {
  const { t } = useTranslation();
  const loginPromptOpen = useSelector<RootState, boolean>((state) => state.main.loginPromptOpen);

  const dispatch = useDispatch();
  const handleClose = () => {
    dispatch(loginPromptToggled());
  };

  const handleLogin = () => {
    dispatch(loginModalOpened());
    handleClose();
  };
  const handleSignup = () => {
    dispatch(signupModalOpened());
    handleClose();
  };

  return (
    <Modal open={loginPromptOpen} onClose={handleClose} noOuterClickClose={false}>
      <div className="modal-card is-compact-mobile is-desktop-style" style={{ minWidth: '300px' }}>
        <div className="modal-card-head">
          <div className="modal-card-title">{t('loginPrompt.title')}</div>
          <ButtonClose onClick={handleClose} />
        </div>
        <div className="modal-card-content flex flex-column">
          <button className="button-main" style={{ marginBottom: '7px' }} onClick={handleLogin}>
            {t('common.login')}
          </button>
          <button onClick={handleSignup}>{t('common.signup')}</button>
        </div>
      </div>
    </Modal>
  );
};

export default LoginPrompt;
