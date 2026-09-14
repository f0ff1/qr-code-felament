import { state, refs } from './state.js';
import { formatProductPrice } from './utils.js';

const PRODUCTS_PER_PAGE = 6;

function productCardHTML(product) {
  return `
    <div class="product-card">
      <strong>${product.name}</strong>
      <span>${product.material} • ${product.estimatedWeight} г • ${product.estimatedPrintTime}</span>
      <span>${product.description}</span>
      <span>${formatProductPrice(product)}</span>
      <div class="card-actions top-gap">
        <button class="mini-btn delete" data-delete-type="product" data-delete-id="${product.id}" aria-label="Удалить продукт" title="Удалить">×</button>
      </div>
    </div>
  `;
}

export function renderProducts() {
  if (!refs.productList) return;

  if (!state.products.length) {
    refs.productList.innerHTML = '<div class="empty-state"><h3>Продукты ещё не добавлены</h3><p>Сначала добавьте продукт в каталог.</p></div>';
    if (refs.productPager) {
      refs.productPager.classList.add('hidden');
      refs.productPager.innerHTML = '';
    }
    return;
  }

  const pages = [];
  for (let i = 0; i < state.products.length; i += PRODUCTS_PER_PAGE) {
    pages.push(state.products.slice(i, i + PRODUCTS_PER_PAGE));
  }
  if (state.productPage >= pages.length) state.productPage = Math.max(0, pages.length - 1);

  refs.productList.className = 'product-list product-carousel';
  refs.productList.innerHTML = `
    <div class="product-carousel-track" style="transform: translateX(-${state.productPage * 100}%)">
      ${pages.map((page) => `
        <div class="product-page">
          ${page.map(productCardHTML).join('')}
        </div>
      `).join('')}
    </div>
  `;

  if (!refs.productPager) return;
  if (pages.length <= 1) {
    refs.productPager.classList.add('hidden');
    refs.productPager.innerHTML = '';
    return;
  }

  refs.productPager.classList.remove('hidden');
  refs.productPager.innerHTML = `
    <button type="button" class="product-page-btn" data-product-page-dir="-1" ${state.productPage <= 0 ? 'disabled' : ''}>‹</button>
    ${pages.map((_, index) => `
      <button type="button" class="product-page-dot ${index === state.productPage ? 'active' : ''}" data-product-page="${index}" aria-label="Страница ${index + 1}"></button>
    `).join('')}
    <button type="button" class="product-page-btn" data-product-page-dir="1" ${state.productPage >= pages.length - 1 ? 'disabled' : ''}>›</button>
  `;
}

export function setProductPage(page) {
  const totalPages = Math.max(1, Math.ceil(state.products.length / PRODUCTS_PER_PAGE));
  state.productPage = Math.max(0, Math.min(totalPages - 1, page));
  renderProducts();
}
