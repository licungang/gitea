import $ from 'jquery';

let ariaIdCounter = 0;

function generateAriaId() {
  return `_aria_auto_id_${ariaIdCounter++}`;
}

// make the item has role=option, and add an id if there wasn't one yet.
function prepareMenuItem($item) {
  if (!$item.attr('id')) $item.attr('id', generateAriaId());
  $item.attr({'role': 'menuitem', 'tabindex': '-1'});
  $item.find('a').attr('tabindex', '-1'); // as above, the elements inside the dropdown menu item should not be focusable, the focus should always be on the dropdown primary element.
}

// when the menu items are loaded from AJAX requests, the items are created dynamically
const defaultCreateDynamicMenu = $.fn.dropdown.settings.templates.menu;
$.fn.dropdown.settings.templates.menu = function(response, fields, preserveHTML, className) {
  const ret = defaultCreateDynamicMenu(response, fields, preserveHTML, className);
  const $wrapper = $('<div>').append(ret);
  const $items = $wrapper.find('> .item');
  $items.each((_, item) => {
    prepareMenuItem($(item));
  });
  return $wrapper.html();
};

function attachOneDropdownAria($dropdown) {
  if ($dropdown.attr('data-aria-attached')) return;
  $dropdown.attr('data-aria-attached', 1);

  const $textSearch = $dropdown.find('input.search').eq(0);
  const $focusable = $textSearch.length ? $textSearch : $dropdown; // see comment below
  if (!$focusable.length) return;

  // prepare menu list
  const $menu = $dropdown.find('> .menu');
  if (!$menu.attr('id')) $menu.attr('id', generateAriaId());

  // dropdown has 2 different focusing behaviors
  // * with search input: the input is focused, and it works perfectly with aria-activedescendant pointing another sibling element.
  // * without search input (but the readonly text), the dropdown itself is focused. then the aria-activedescendant points to the element inside dropdown

  $focusable.attr({
    'role': 'menu',
    'aria-haspopup': 'menu',
    'aria-controls': $menu.attr('id'),
    'aria-expanded': 'false',
  });

  if ($dropdown.attr('data-content') && !$dropdown.attr('aria-label')) {
    $dropdown.attr('aria-label', $dropdown.attr('data-content'));
  }

  $menu.find('> .item').each((_, item) => {
    prepareMenuItem($(item));
  });

  // update aria attributes according current active/selected item
  const refreshAria = () => {
    const isMenuVisible = !$menu.is('.hidden') && !$menu.is('.animating.out');
    $focusable.attr('aria-expanded', isMenuVisible ? 'true' : 'false');

    let $active = $menu.find('> .item.active');
    if (!$active.length) $active = $menu.find('> .item.selected'); // it's strange that we need this fallback at the moment

    // if there is an active item, use its id. if no active item, then the empty string is set
    $focusable.attr('aria-activedescendant', $active.attr('id'));
  };

  // use setTimeout to run the refreshAria in next tick
  $focusable.on('focus', () => {
    setTimeout(refreshAria, 0);
  });
  $focusable.on('mouseup', () => {
    setTimeout(refreshAria, 0);
  });
  $focusable.on('blur', () => {
    setTimeout(refreshAria, 0);
  });
  $dropdown.on('keyup', (e) => {
    const key = e.key;
    if (key === 'Tab' || key === 'Space' || key === 'Enter' || key.startsWith('Arrow')) {
      setTimeout(refreshAria, 0);
    }
  });
}

export function attachDropdownAria($dropdowns) {
  $dropdowns.each((_, e) => attachOneDropdownAria($(e)));
}
