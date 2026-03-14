import { Helmet } from 'react-helmet-async';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import Sidebar from '../components/Sidebar';
import { useRemoveCanonicalTag } from '../hooks';

const NotFound = () => {
  const { t } = useTranslation();
  useRemoveCanonicalTag();
  return (
    <div className="page-content page-notfound">
      <Helmet>
        <title>{t('notFound.title')}</title>
        <meta name="robots" content="noindex" />
      </Helmet>
      <Sidebar />
      <h1>{t('notFound.heading')}</h1>
      <p>{t('notFound.message')}</p>
      <Link to="/">{t('notFound.goHome')}</Link>
    </div>
  );
};

export default NotFound;
