import { setJsonLd, setPageSeo, setMetaTag } from '../../../shared/seo';
import { ArticleMeta, articleUrl, SITE_URL } from './registry';

const FALLBACK_IMAGE = `${SITE_URL}/og-cover.jpg`;

/** Social cards and Article structured data need a raster image. */
export function articleImageUrl(meta: ArticleMeta): string {
  return /\.(png|jpe?g|webp)$/i.test(meta.cover) ? `${SITE_URL}${meta.cover}` : FALLBACK_IMAGE;
}

/** Title, description, canonical, social tags, Article and Breadcrumb data for
 * one journal article. Every article page calls this, old and new alike. */
export function setArticleSeo(doc: Document, meta: ArticleMeta): void {
  const url = articleUrl(meta.slug);
  setPageSeo(doc, { title: `${meta.title} | Indifferent`, description: meta.description, canonical: url });
  setMetaTag(doc, 'property', 'og:type', 'article');
  setMetaTag(doc, 'property', 'og:image', articleImageUrl(meta));
  setMetaTag(doc, 'name', 'twitter:image', articleImageUrl(meta));
  setJsonLd(doc, 'article', {
    '@context': 'https://schema.org',
    '@type': 'Article',
    headline: meta.title,
    description: meta.description,
    datePublished: meta.published,
    dateModified: meta.updated,
    image: articleImageUrl(meta),
    mainEntityOfPage: url,
    author: { '@type': 'Organization', name: 'Indifferent', url: SITE_URL },
    publisher: {
      '@type': 'Organization',
      name: 'Indifferent',
      logo: { '@type': 'ImageObject', url: `${SITE_URL}/icons/icon-512x512.png` },
    },
  });
  setJsonLd(doc, 'breadcrumbs', {
    '@context': 'https://schema.org',
    '@type': 'BreadcrumbList',
    itemListElement: [
      { '@type': 'ListItem', position: 1, name: 'Home', item: `${SITE_URL}/` },
      { '@type': 'ListItem', position: 2, name: 'Journal', item: `${SITE_URL}/blog` },
      { '@type': 'ListItem', position: 3, name: meta.title, item: url },
    ],
  });
}
