import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useDispatch } from 'react-redux';
import { useLocation } from 'react-router-dom';
import { Form, FormField } from '../components/Form';
import Input, { InputPassword } from '../components/Input';
import { APIError, mfetch } from '../helper';
import { loginModalOpened, signupModalOpened, snackAlertError } from '../slices/mainSlice';

const LoginForm = ({ isModal = false }: { isModal?: boolean }) => {
  const { t } = useTranslation();
  const dispatch = useDispatch();

  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [loginError, setLoginError] = useState<string | null>(null);
  useEffect(() => {
    setLoginError(null);
  }, [username, password]);
  const handleLoginSubmit: React.FormEventHandler = async (event) => {
    event.preventDefault();
    if (username === '' && password === '') {
      setLoginError(t('auth:usernameAndPasswordEmpty'));
      return;
    } else if (username === '') {
      setLoginError(t('auth:usernameEmptyShort'));
      return;
    } else if (password === '') {
      setLoginError(t('auth:passwordEmptyShort'));
      return;
    }
    try {
      const res = await mfetch('/api/_login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json; charset=utf-8',
        },
        body: JSON.stringify({ username, password }),
      });
      if (res.ok) {
        window.location.reload();
      } else {
        if (res.status === 401) {
          setLoginError(t('auth:usernamePasswordNoMatch'));
        } else if (res.status === 403) {
          const json = await res.json();
          if (json.code === 'account_suspended') {
            setLoginError(t('auth:accountSuspended', { username }));
          } else {
            throw new APIError(res.status, json);
          }
        } else {
          throw new APIError(res.status, await res.json());
        }
      }
    } catch (error) {
      dispatch(snackAlertError(error));
    }
  };

  const usernameRef = useRef<HTMLInputElement>(null);
  const { pathname } = useLocation();
  useEffect(() => {
    if (pathname === '/login') {
      usernameRef.current?.focus();
    }
  }, [pathname]);

  const handleOnSignup: React.MouseEventHandler = (event) => {
    event.preventDefault();
    dispatch(loginModalOpened(false));
    dispatch(signupModalOpened());
  };

  return (
    <Form className="login-box modal-card-content" onSubmit={handleLoginSubmit}>
      <FormField label={t('username')}>
        <Input
          ref={usernameRef}
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          autoFocus={isModal}
          autoComplete="username"
        />
      </FormField>
      <FormField label={t('password')}>
        <InputPassword
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete="current-password"
        />
      </FormField>
      {loginError && (
        <FormField>
          <div className="form-error text-center">{loginError}</div>
        </FormField>
      )}
      <FormField className="is-submit">
        <input type="submit" className="button button-main" value={t('login')} />
        <button className="button-link" onClick={handleOnSignup}>
          {t('auth:noAccountSignup')}
        </button>
      </FormField>
    </Form>
  );
};

export default LoginForm;
