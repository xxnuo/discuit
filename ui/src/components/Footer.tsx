import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import Link from './Link';

const Footer = () => {
  const { t } = useTranslation();
  const className = 'footer';

  useEffect(() => {
    const footerEl = document.querySelector(className);
    if (footerEl) {
      const background = document.documentElement.style.background;
      document.documentElement.style.background = window.getComputedStyle(footerEl).background;
      return () => {
        document.documentElement.style.background = background;
      };
    }
  }, []);

  return (
    <footer className={className}>
      <div className="wrap">
        <div className="footer-col footer-show">
          <Link to="/" className="footer-logo">
            {import.meta.env.VITE_SITENAME}
          </Link>
          <div className="footer-description">{t('footer.description')}</div>
        </div>
        <div className="footer-col">
          <div className="footer-title">{t('footer.organization')}</div>
          <Link to="/about" className="footer-item">
            {t('sidebar.about')}
          </Link>
          <a href={`mailto:${import.meta.env.VITE_EMAILCONTACT}`} className="footer-item">
            {t('sidebar.contact')}
          </a>
        </div>
        <div className="footer-col">
          <div className="footer-title">{t('footer.social')}</div>
          {import.meta.env.VITE_TWITTERURL && (
            <a
              href={import.meta.env.VITE_TWITTERURL}
              className="footer-item"
              target="_blank"
              rel="noopener"
            >
              Twitter / X
            </a>
          )}
          {import.meta.env.VITE_SUBSTACKURL && (
            <a
              href={import.meta.env.VITE_SUBSTACKURL}
              className="footer-item"
              target="_blank"
              rel="noopener"
            >
              {t('footer.blog')}
            </a>
          )}
          {import.meta.env.VITE_FACEBOOKURL && (
            <a
              href={import.meta.env.VITE_FACEBOOKURL}
              className="footer-item"
              target="_blank"
              rel="noopener"
            >
              Facebook
            </a>
          )}
          {import.meta.env.VITE_INSTAGRAMURL && (
            <a
              href={import.meta.env.VITE_INSTAGRAMURL}
              className="footer-item"
              target="_blank"
              rel="noopener"
            >
              Instagram
            </a>
          )}
          {import.meta.env.VITE_DISCORDURL && (
            <a
              href={import.meta.env.VITE_DISCORDURL}
              className="footer-item"
              target="_blank"
              rel="noopener"
            >
              Discord
            </a>
          )}
          {import.meta.env.VITE_GITHUBURL && (
            <a
              href={import.meta.env.VITE_GITHUBURL}
              className="footer-item"
              target="_blank"
              rel="noopener"
            >
              Github
            </a>
          )}
        </div>
        <div className="footer-col">
          <div className="footer-title">{t('footer.policies')}</div>
          <Link className="footer-item" to="/guidelines">
            {t('footer.siteGuidelines')}
          </Link>
          <Link className="footer-item" to="/moderator-guidelines">
            {t('footer.moderatorGuidelines')}
          </Link>
          <Link className="footer-item" to="/terms">
            {t('sidebar.terms')}
          </Link>
          <Link className="footer-item" to="/privacy-policy">
            {t('sidebar.privacy')}
          </Link>
          <a
            className="footer-item"
            href="https://docs.discuit.org/"
            target="_blank"
            rel="noopener"
          >
            {t('footer.documentation')}
          </a>
        </div>
      </div>
    </footer>
  );
};

export default Footer;
