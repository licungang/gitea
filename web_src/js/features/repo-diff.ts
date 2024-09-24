import $ from 'jquery';
import {initCompReactionSelector} from './comp/ReactionSelector.ts';
import {initRepoIssueContentHistory} from './repo-issue-content.ts';
import {initDiffFileTree} from './repo-diff-filetree.ts';
import {initDiffCommitSelect} from './repo-diff-commitselect.ts';
import {validateTextareaNonEmpty} from './comp/ComboMarkdownEditor.ts';
import {initViewedCheckboxListenerFor, countAndUpdateViewedFiles, initExpandAndCollapseFilesButton} from './pull-view-file.ts';
import {initImageDiff} from './imagediff.ts';
import {showErrorToast} from '../modules/toast.ts';
import {submitEventSubmitter, queryElemSiblings, hideElem, showElem, animateOnce} from '../utils/dom.ts';
import {POST, GET} from '../modules/fetch.ts';

const {pageData, i18n} = window.config;

function initRepoDiffReviewButton() {
  const reviewBox = document.querySelector('#review-box');
  if (!reviewBox) return;

  const counter = reviewBox.querySelector('.review-comments-counter');
  if (!counter) return;

  function handleFormSubmit(form, textarea) {
    if (form.getAttribute('data-handler-attached') === 'true') return;
    form.setAttribute('data-handler-attached', 'true');
    form.addEventListener('submit', (event) => {
      if (textarea.value.trim() === '') {
        event.preventDefault();
        return;
      }
      const num = (parseInt(counter.getAttribute('data-pending-comment-number')) || 0) + 1;
      counter.setAttribute('data-pending-comment-number', num);
      counter.textContent = num;
      animateOnce(reviewBox, 'pulse-1p5-200');
      form.removeAttribute('data-handler-attached');
    });
  }

  // Handle submit on click
  document.addEventListener('click', (e) => {
    if (e.target.name === 'pending_review') {
      const form = e.target.closest('form');
      const textarea = form.querySelector('textarea');
      handleFormSubmit(form, textarea);
    }
  });

  // Handle submit by ctrl+enter
  document.addEventListener('keydown', (e) => {
    if (e.ctrlKey && e.key === 'Enter') {
      const textarea = e.target;
      if (textarea.tagName.toLowerCase() === 'textarea') {
        const form = textarea.closest('form');
        handleFormSubmit(form, textarea);
      }
    }
  });
}

function initRepoDiffFileViewToggle() {
  $('.file-view-toggle').on('click', function () {
    for (const el of queryElemSiblings(this)) {
      el.classList.remove('active');
    }
    this.classList.add('active');

    const target = document.querySelector(this.getAttribute('data-toggle-selector'));
    if (!target) return;

    hideElem(queryElemSiblings(target));
    showElem(target);
  });
}

function initRepoDiffConversationForm() {
  $(document).on('submit', '.conversation-holder form', async (e) => {
    e.preventDefault();

    const $form = $(e.target);
    const textArea = e.target.querySelector('textarea');
    if (!validateTextareaNonEmpty(textArea)) {
      return;
    }

    if (e.target.classList.contains('is-loading')) return;
    try {
      e.target.classList.add('is-loading');
      const formData = new FormData($form[0]);

      // if the form is submitted by a button, append the button's name and value to the form data
      const submitter = submitEventSubmitter(e);
      const isSubmittedByButton = (submitter?.nodeName === 'BUTTON') || (submitter?.nodeName === 'INPUT' && submitter.type === 'submit');
      if (isSubmittedByButton && submitter.name) {
        formData.append(submitter.name, submitter.value);
      }

      const response = await POST(e.target.getAttribute('action'), {data: formData});
      const $newConversationHolder = $(await response.text());
      const {path, side, idx} = $newConversationHolder.data();

      $form.closest('.conversation-holder').replaceWith($newConversationHolder);
      let selector;
      if ($form.closest('tr').data('line-type') === 'same') {
        selector = `[data-path="${path}"] .add-code-comment[data-idx="${idx}"]`;
      } else {
        selector = `[data-path="${path}"] .add-code-comment[data-side="${side}"][data-idx="${idx}"]`;
      }
      for (const el of document.querySelectorAll(selector)) {
        el.classList.add('tw-invisible');
      }
      $newConversationHolder.find('.dropdown').dropdown();
    } catch (error) {
      console.error('Error:', error);
      showErrorToast(i18n.network_error);
    } finally {
      e.target.classList.remove('is-loading');
    }
  });

  $(document).on('click', '.resolve-conversation', async function (e) {
    e.preventDefault();
    const comment_id = $(this).data('comment-id');
    const origin = $(this).data('origin');
    const action = $(this).data('action');
    const url = $(this).data('update-url');

    try {
      const response = await POST(url, {data: new URLSearchParams({origin, action, comment_id})});
      const data = await response.text();

      if ($(this).closest('.conversation-holder').length) {
        const $conversation = $(data);
        $(this).closest('.conversation-holder').replaceWith($conversation);
        $conversation.find('.dropdown').dropdown();
        initCompReactionSelector($conversation);
      } else {
        window.location.reload();
      }
    } catch (error) {
      console.error('Error:', error);
    }
  });
}

export function initRepoDiffConversationNav() {
  // Previous/Next code review conversation
  $(document).on('click', '.previous-conversation', (e) => {
    const $conversation = $(e.currentTarget).closest('.comment-code-cloud');
    const $conversations = $('.comment-code-cloud:not(.tw-hidden)');
    const index = $conversations.index($conversation);
    const previousIndex = index > 0 ? index - 1 : $conversations.length - 1;
    const $previousConversation = $conversations.eq(previousIndex);
    const anchor = $previousConversation.find('.comment').first()[0].getAttribute('id');
    window.location.href = `#${anchor}`;
  });
  $(document).on('click', '.next-conversation', (e) => {
    const $conversation = $(e.currentTarget).closest('.comment-code-cloud');
    const $conversations = $('.comment-code-cloud:not(.tw-hidden)');
    const index = $conversations.index($conversation);
    const nextIndex = index < $conversations.length - 1 ? index + 1 : 0;
    const $nextConversation = $conversations.eq(nextIndex);
    const anchor = $nextConversation.find('.comment').first()[0].getAttribute('id');
    window.location.href = `#${anchor}`;
  });
}

// Will be called when the show more (files) button has been pressed
function onShowMoreFiles() {
  initRepoIssueContentHistory();
  initViewedCheckboxListenerFor();
  countAndUpdateViewedFiles();
  initImageDiff();
}

export async function loadMoreFiles(url) {
  const target = document.querySelector('a#diff-show-more-files');
  if (target?.classList.contains('disabled') || pageData.diffFileInfo.isLoadingNewData) {
    return;
  }

  pageData.diffFileInfo.isLoadingNewData = true;
  target?.classList.add('disabled');

  try {
    const response = await GET(url);
    const resp = await response.text();
    const $resp = $(resp);
    // the response is a full HTML page, we need to extract the relevant contents:
    // 1. append the newly loaded file list items to the existing list
    $('#diff-incomplete').replaceWith($resp.find('#diff-file-boxes').children());
    // 2. re-execute the script to append the newly loaded items to the JS variables to refresh the DiffFileTree
    $('body').append($resp.find('script#diff-data-script'));

    onShowMoreFiles();
  } catch (error) {
    console.error('Error:', error);
    showErrorToast('An error occurred while loading more files.');
  } finally {
    target?.classList.remove('disabled');
    pageData.diffFileInfo.isLoadingNewData = false;
  }
}

function initRepoDiffShowMore() {
  $(document).on('click', 'a#diff-show-more-files', (e) => {
    e.preventDefault();

    const linkLoadMore = e.target.getAttribute('data-href');
    loadMoreFiles(linkLoadMore);
  });

  $(document).on('click', 'a.diff-load-button', async (e) => {
    e.preventDefault();
    const $target = $(e.target);

    if (e.target.classList.contains('disabled')) {
      return;
    }

    e.target.classList.add('disabled');

    const url = $target.data('href');

    try {
      const response = await GET(url);
      const resp = await response.text();

      if (!resp) {
        return;
      }
      $target.parent().replaceWith($(resp).find('#diff-file-boxes .diff-file-body .file-body').children());
      onShowMoreFiles();
    } catch (error) {
      console.error('Error:', error);
    } finally {
      e.target.classList.remove('disabled');
    }
  });
}

export function initRepoDiffView() {
  initRepoDiffConversationForm();
  if (!$('#diff-file-list').length) return;
  initDiffFileTree();
  initDiffCommitSelect();
  initRepoDiffShowMore();
  initRepoDiffReviewButton();
  initRepoDiffFileViewToggle();
  initViewedCheckboxListenerFor();
  initExpandAndCollapseFilesButton();
}
