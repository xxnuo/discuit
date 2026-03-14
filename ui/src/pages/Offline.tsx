import { useTranslation } from 'react-i18next';
import Navbar from '../components/Navbar';

const Offline = () => {
  const { t } = useTranslation();
  const handleRetry = () => window.location.reload();
  return (
    <>
      <Navbar offline />
      <div className="page-content page-notfound page-offline">
        <h1>{t('offline.title')}</h1>
        <p>{t('offline.message')}</p>
        <button onClick={handleRetry}>{t('retry')}</button>
      </div>
    </>
  );
};

export default Offline;
