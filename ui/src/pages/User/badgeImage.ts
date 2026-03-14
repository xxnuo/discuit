export function badgeImage(type: string): { src: string; alt: string } {
  const ret = {
    src: '',
    alt: '',
  };
  switch (type) {
    case 'supporter':
      ret.src = '/badge-supporter.png';
      ret.alt = 'supporter badge';
      break;
    default:
      throw new Error(`unknown badge type '${type}'`);
  }
  return ret;
}
