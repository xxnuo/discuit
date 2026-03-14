import { useSelector } from 'react-redux';
import { Redirect } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { MainState } from '../slices/mainSlice';
import { RootState } from '../store';
import LoginForm from '../views/LoginForm';

const Login = () => {
  const { t } = useTranslation('common');
  const user = useSelector<RootState>((state) => state.main.user) as MainState['user'];
  const loggedIn = user !== null;

  if (loggedIn) {
    return <Redirect to="/" />;
  }

  return (
    <div className="page-content page-login wrap">
      <div className="card login-card">
        <div className="title">{t('loginPrompt.title')}</div>
        <LoginForm />
      </div>
    </div>
  );
};

export default Login;
