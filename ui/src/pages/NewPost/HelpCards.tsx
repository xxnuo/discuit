import { useTranslation } from 'react-i18next';

export const HelpCardCommunity = () => {
  const { t } = useTranslation('post');
  return (
    <div className="newpost-help card-gray">
      <div className="newpost-help-title">{t('selectCommunity')}</div>
      <p>{t('helpSelectCommunity')}</p>
    </div>
  );
};

export const HelpCardBody = () => {
  const { t } = useTranslation('post');
  return (
    <div className="newpost-help card-gray">
      <p>{t('helpMarkdown')}</p>
    </div>
  );
};

export const HelpCardTitle = () => {
  const { t } = useTranslation('post');
  return (
    <div className="newpost-help card-gray">
      <p>{t('helpTitleLimit')}</p>
    </div>
  );
};
