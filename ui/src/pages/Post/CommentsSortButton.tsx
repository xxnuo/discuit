import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import Dropdown from '../../components/Dropdown';

const CommentsSortButton = ({
  defaultSort = 'Top',
  onSortChange,
}: {
  defaultSort: string;
  onSortChange: (sort: string) => void;
}) => {
  const { t } = useTranslation('post');
  const options = [t('sortTop'), t('sortLatest'), t('sortOldest')];
  const [sort, _setSort] = useState(defaultSort);
  const setSort = (newVal: string) => {
    _setSort(newVal);
    if (onSortChange) onSortChange(newVal);
  };

  return (
    <div className="post-comments-sort">
      <Dropdown
        target={<div className="button button-text">{t('sortBy', { sort })}</div>}
        aligned="right"
      >
        <div className="dropdown-list">
          {options.map((opt) => (
            <button key={opt} className="button-clear dropdown-item" onClick={() => setSort(opt)}>
              {opt}
            </button>
          ))}
        </div>
      </Dropdown>
    </div>
  );
};

export default CommentsSortButton;
