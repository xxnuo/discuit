import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { useDispatch } from 'react-redux';
import Dropdown from '../../components/Dropdown';
import { copyToClipboard, publicURL } from '../../helper';
import { snackAlert } from '../../slices/mainSlice';

export const CommentShareDropdownItems = ({ url }: { url: string }) => {
  const dispatch = useDispatch();
  const { t } = useTranslation('post');
  const handleCopyURL = () => {
    let text = t('linkCopyFailed');
    if (copyToClipboard(publicURL(url))) {
      text = t('linkCopied');
    }
    dispatch(snackAlert(text, 'comment_link_copied'));
  };

  return (
    <div className="dropdown-item" onClick={handleCopyURL}>
      {t('copyUrl')}
    </div>
  );
};

CommentShareDropdownItems.propTypes = {
  url: PropTypes.string.isRequired,
};

const CommentShareButton = ({ url }: { url: string }) => {
  const { t } = useTranslation('common');
  return (
    <Dropdown target={<button className="button-text post-comment-button">{t('share')}</button>}>
      <div className="dropdown-list">
        <CommentShareDropdownItems url={url} />
      </div>
    </Dropdown>
  );
};

export default CommentShareButton;
