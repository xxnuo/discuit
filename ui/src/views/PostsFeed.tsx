import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useDispatch, useSelector } from 'react-redux';
import { useHistory, useLocation } from 'react-router';
import Feed from '../components/Feed';
import { MemorizedPostCard } from '../components/PostCard/PostCard';
import SelectBar from '../components/SelectBar';
import { mfetchjson } from '../helper';
import { useCanonicalTag } from '../hooks';
import { isInfiniteScrollingDisabled } from '../pages/Settings/devicePrefs';
import { FeedItem, feedReloaded } from '../slices/feedsSlice';
import { MainState } from '../slices/mainSlice';
import { Post } from '../slices/postsSlice';
import { RootState } from '../store';
import WelcomeBanner from './WelcomeBanner';

type SortOption = {
  text: string;
  id: string;
  to?: string;
};

const sortOptionKeys: { textKey: string; id: string; to?: string }[] = [
  { textKey: 'feed.hot', id: 'hot' },
  { textKey: 'feed.activity', id: 'activity' },
  { textKey: 'feed.new', id: 'latest' },
  { textKey: 'feed.topDay', id: 'day' },
  { textKey: 'feed.topWeek', id: 'week' },
  { textKey: 'feed.topMonth', id: 'month' },
  { textKey: 'feed.topYear', id: 'year' },
  { textKey: 'feed.topAll', id: 'all' },
];
const sortDefault = import.meta.env.VITE_DEFAULTFEEDSORT;
const baseURL = '/api/posts';

export const homeReloaded = (homeFeed = 'all', rememberFeedSort = false) => {
  const params = new URLSearchParams();
  let sort = sortDefault;
  if (rememberFeedSort) {
    sort = window.localStorage.getItem('feedSort') || sortDefault;
  }
  params.set('sort', sort);
  if (homeFeed === 'subscriptions') params.set('feed', 'home');
  return feedReloaded(`${baseURL}?${params.toString()}`);
};

function useFeedSort(rememberLastSort = false): [string | null, (newSort: string) => void] {
  const location = useLocation();

  let sortSaved: string | null = null;
  if (rememberLastSort) {
    sortSaved = window.localStorage.getItem('feedSort');
  } else {
    const params = new URLSearchParams(location.search);
    sortSaved = params.get('sort');
  }
  sortSaved = sortSaved || sortDefault;

  // If sortOptions.to is preset, use query parameters and go to the link.
  for (let i = 0; i < sortOptionKeys.length; i++) {
    if (rememberLastSort) {
      sortOptionKeys[i].to = undefined;
    } else {
      if (sortOptionKeys[i].id === sortDefault) {
        sortOptionKeys[i].to = location.pathname;
      } else {
        sortOptionKeys[i].to = `${location.pathname}?sort=${sortOptionKeys[i].id}`;
      }
    }
  }

  const history = useHistory();
  const [sort, _setSort] = useState(sortSaved);

  const setSort = (newSort: string) => {
    _setSort(newSort);
    if (rememberLastSort) {
      window.localStorage.setItem('feedSort', newSort);
    } else {
      let to = '#';
      sortOptionKeys
        .filter((option) => option.id === newSort)
        .forEach((option) => (to = option.to || ''));
      history.replace(to);
    }
  };

  useEffect(() => {
    if (!rememberLastSort) {
      const params = new URLSearchParams(location.search);
      _setSort(params.get('sort') || sortDefault);
    }
  }, [location, rememberLastSort]);

  return [sort, setSort];
}

export type PostsFeedType = 'all' | 'subscriptions' | 'community' | 'moderating';

const PostsFeed = ({
  feedType = 'all',
  communityId = null,
}: {
  feedType: PostsFeedType;
  communityId: string | null;
}) => {
  const dispatch = useDispatch();

  const { t } = useTranslation();

  const sortOptions: SortOption[] = sortOptionKeys.map((o) => ({
    text: t(o.textKey),
    id: o.id,
    to: o.to,
  }));

  const user = useSelector<RootState>((state) => state.main.user) as MainState['user'];
  const loggedIn = user !== null;

  const location = useLocation();
  const [sort, setSort] = useFeedSort(Boolean(user && user.rememberFeedSort));

  // The ordering of the urlparams here is important because the items are
  // stored by the url as the key.
  const urlParams = new URLSearchParams();
  urlParams.set('sort', sort || '');
  if (loggedIn) {
    switch (feedType) {
      case 'subscriptions':
        urlParams.set('feed', 'home');
        break;
      case 'moderating':
        urlParams.set('feed', 'moderating');
        break;
    }
  }
  if (communityId !== null) urlParams.set('communityId', communityId);
  const feedId = `${baseURL}?${urlParams.toString()}`; // api endpoint.

  // Only called on button clicks (not history API changes)
  const handleSortChange = (optionId: string) => {
    setSort(optionId);
    dispatch(feedReloaded(feedId));
  };

  const layout = useSelector<RootState>(
    (state) => state.main.feedLayout
  ) as MainState['feedLayout'];
  const compact = layout === 'compact';

  const handleRenderItem = (item: FeedItem<Post>, index: number) => {
    return (
      <MemorizedPostCard
        initialPost={item.item}
        index={index}
        disableEmbeds={Boolean(user && user.embedsOff)}
        compact={compact}
        feedItemKey={item.key}
        canHideFromFeed
      />
    );
  };

  const canonicalURL = () => {
    const sortValid = sortOptionKeys.filter((option) => option.id === sort).length !== 0;
    if (!sortValid) return '';
    const url = window.location;
    const search = sort === sortDefault ? '' : `?sort=${sort}`;
    return url.origin + url.pathname + search;
  };
  useCanonicalTag(canonicalURL(), [location]);

  let name = t('feed.posts');
  if (!communityId) {
    if (feedType === 'all') {
      name = t('feed.home');
    } else if (feedType === 'subscriptions') {
      name = t('feed.subscriptions');
    } else if (feedType === 'moderating') {
      name = t('feed.moderating');
    }
  }

  const handleFetch = async (next?: string | null) => {
    const params = new URLSearchParams(urlParams.toString());
    if (next) {
      params.set('next', next);
    }
    const url = `${baseURL}?${params.toString()}`;
    const res = (await mfetchjson(url)) as { posts: Post[] | null; next: string | null };
    const feedItems: FeedItem<Post>[] = (res.posts ?? []).map(
      (post) => new FeedItem(post, 'post', post.publicId)
    );
    return {
      items: feedItems,
      next: res.next,
    };
  };

  return (
    <div className="posts-feed">
      <SelectBar name={name} options={sortOptions} value={sort || ''} onChange={handleSortChange} />
      <Feed<Post>
        feedId={feedId}
        compact={compact}
        onFetch={handleFetch}
        onRenderItem={handleRenderItem}
        banner={!loggedIn ? <WelcomeBanner className="is-m is-in-feed" /> : null}
        infiniteScrollingDisabled={isInfiniteScrollingDisabled()}
      />
    </div>
  );
};

export default PostsFeed;
