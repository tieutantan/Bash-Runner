// Giao diện Bash Runner. Backend Go được Wails gắn vào window.go.main.App, sự kiện qua window.runtime.
(function () {
  'use strict';

  const api = () => window.go && window.go.main && window.go.main.App;
  const $ = (id) => document.getElementById(id);

  const svg = (body, size = 16, extra = '') =>
    `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" ${extra}>${body}</svg>`;
  const solid = (body, size = 16) =>
    `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">${body}</svg>`;

  const ICON = {
    play: (s) => solid('<path d="M7 4.5v15l12-7.5z"/>', s),
    more: (s) => solid('<circle cx="12" cy="5" r="1.8"/><circle cx="12" cy="12" r="1.8"/><circle cx="12" cy="19" r="1.8"/>', s),
    file: (s) => svg('<path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z"/><path d="M14 3v5h5"/>', s),
    warn: (s) => svg('<path d="M12 3l10 18H2z"/><path d="M12 10v5M12 18.2v.1"/>', s),
    pencil: (s) => svg('<path d="M4 20h4L19 9l-4-4L4 16z"/><path d="M13.5 6.5l4 4"/>', s),
    copy: (s) => svg('<rect x="9" y="9" width="11" height="11" rx="2"/><path d="M5 15V6a2 2 0 0 1 2-2h8"/>', s),
    external: (s) => svg('<path d="M14 4h6v6M20 4l-9 9"/><path d="M18 14v4a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4"/>', s),
    trash: (s) => svg('<path d="M5 7h14M10 11v6M14 11v6M6 7l1 12a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-12M9 7V4h6v3"/>', s),
    close: (s) => svg('<path d="M6 6l12 12M18 6L6 18"/>', s),
    spinner: (s) => svg('<path d="M12 3a9 9 0 1 0 9 9"/>', s, 'class="spin"'),
    upload: (s) => svg('<path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z"/><path d="M14 3v5h5M12 17v-5M9.5 14.5L12 12l2.5 2.5"/>', s),
    alert: (s) => svg('<circle cx="12" cy="12" r="9"/><path d="M12 7v6M12 16.5v.1"/>', s),
  };

  const state = {
    scripts: null, // null = đang tải
    loadError: '',
    running: new Set(),
    lastResult: null, // null | 'ok' | 'fail'
  };

  const esc = (s) =>
    String(s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const runKey = (path, params) => path + '\x00' + params;
  const plural = (n) => `${n} file bash`;

  // ---------- Danh sách ----------

  function setScripts(list) {
    state.scripts = list || [];
    state.loadError = '';
    render();
  }

  function render() {
    const list = $('list');
    if (state.loadError) {
      list.innerHTML = `<div class="state"><div class="big bad">${ICON.alert(30)}</div>
        <h2>Không tải được danh sách</h2><p>${esc(state.loadError)}</p>
        <button class="btn btn-primary" type="button" data-action="retry">Thử lại</button></div>`;
      $('count').textContent = '';
      return;
    }
    if (state.scripts === null) {
      list.innerHTML = '<div class="skel"></div><div class="skel"></div>';
      $('count').innerHTML = '&nbsp;';
      return;
    }
    $('count').textContent = state.scripts.length ? plural(state.scripts.length) : 'Chưa có file';
    if (!state.scripts.length) {
      list.innerHTML = `<div class="state"><div class="big">${ICON.upload(30)}</div>
        <h2>Chưa có file nào</h2><p>Chọn file .sh ở bất kỳ đâu trên máy để chạy bằng một cú bấm.</p>
        <button class="btn btn-primary" type="button" data-action="add">Thêm file</button>
        <div class="steps"><span><b>1</b> Thêm file</span><span><b>2</b> Bấm Chạy</span><span><b>3</b> Xem output</span></div></div>`;
      return;
    }
    list.innerHTML = state.scripts.map(cardHTML).join('');
  }

  function runButton(s, params, small) {
    const busy = state.running.has(runKey(s.path, params));
    const cls = 'run' + (small ? ' sm' : '');
    const size = small ? 12 : 14;
    const data = `data-path="${esc(s.path)}" data-params="${esc(params)}"`;
    if (busy) return `<button class="${cls}" type="button" disabled aria-busy="true">${ICON.spinner(size)}Đang chạy</button>`;
    if (!s.exists) return `<button class="${cls}" type="button" disabled title="Không tìm thấy file">${ICON.play(size)}Chạy</button>`;
    return `<button class="${cls}" type="button" data-action="run" ${data}>${ICON.play(size)}Chạy</button>`;
  }

  function cardHTML(s, i) {
    const sub = s.exists
      ? (s.runs.length ? `${s.runs.length} lần chạy đã lưu` : '')
      : 'Không tìm thấy file · ' + esc(s.path);
    const runs = s.runs
      .map(
        (r, j) => `<div class="sub-run">
          <div class="grow"><div class="name">${esc(r.name)}</div><div class="code">${esc(r.params)}</div></div>
          ${runButton(s, r.params, true)}
          <button class="more" type="button" data-action="menu-run" data-i="${i}" data-j="${j}" aria-haspopup="menu" aria-expanded="false" aria-label="Tùy chọn ${esc(r.name)}">${ICON.more(20)}</button>
        </div>`
      )
      .join('');
    return `<article class="card">
      <div class="row">
        <div class="tile ${s.exists ? '' : 'tile-danger'}">${s.exists ? ICON.file(20) : ICON.warn(20)}</div>
        <div class="grow"><div class="name">${esc(s.displayName)}</div>${sub ? `<div class="path ${s.exists ? '' : 'bad'}">${sub}</div>` : ''}</div>
        ${runButton(s, '', false)}
        <button class="more" type="button" data-action="menu-file" data-i="${i}" aria-haspopup="menu" aria-expanded="false" aria-label="Tùy chọn ${esc(s.displayName)}">${ICON.more(20)}</button>
      </div>${runs}</article>`;
  }

  async function load() {
    state.loadError = '';
    state.scripts = null;
    render();
    try {
      const [list, running] = await Promise.all([api().GetScripts(), api().GetRunning()]);
      state.running = new Set(running || []);
      setScripts(list);
    } catch (e) {
      state.loadError = String(e && e.message ? e.message : e);
      render();
    }
  }

  async function addFile() {
    try {
      setScripts(await api().AddScript());
    } catch (e) {
      toast(String(e));
    }
  }

  // ---------- Menu dropdown ----------

  const menu = $('menu');
  let menuTrigger = null;

  function menuItem(it) {
    const cls = 'mi' + (it.style ? ' ' + it.style : '');
    const head = (it.divider ? '<div class="msep"></div>' : '') + (it.group ? `<div class="grp">${esc(it.group)}</div>` : '');
    return `${head}<button class="${cls}" type="button" role="menuitem" data-idx="${it.id}">
      <span class="mic">${it.icon(16)}</span>
      <span><span class="mt">${esc(it.label)}</span><span class="md">${esc(it.desc)}</span></span></button>`;
  }

  // openMenu hiện menu neo vào nút trigger: mép phải thẳng hàng với nút, hết chỗ phía dưới thì mở lên trên.
  function openMenu(trigger, header, items) {
    closeMenu();
    items.forEach((it, id) => (it.id = id));
    menu.setAttribute('aria-label', trigger.getAttribute('aria-label') || 'Tùy chọn');
    menu._items = items;
    menu.innerHTML = header + items.map(menuItem).join('');
    menu.hidden = false;
    menu.classList.remove('up');

    const r = trigger.getBoundingClientRect();
    const w = menu.offsetWidth;
    const h = menu.offsetHeight;
    let left = Math.max(8, Math.min(r.right - w, window.innerWidth - w - 8));
    let top = r.bottom + 4;
    if (top + h > window.innerHeight - 8) {
      top = Math.max(8, r.top - 4 - h);
      menu.classList.add('up');
    }
    menu.style.left = left + 'px';
    menu.style.top = top + 'px';

    trigger.setAttribute('aria-expanded', 'true');
    menuTrigger = trigger;
    menu.querySelector('.mi').focus({ preventScroll: true });
  }

  function closeMenu(restoreFocus) {
    if (menu.hidden) return;
    menu.hidden = true;
    menu.innerHTML = '';
    if (menuTrigger) {
      menuTrigger.setAttribute('aria-expanded', 'false');
      if (restoreFocus) menuTrigger.focus();
      menuTrigger = null;
    }
  }

  function headerHTML(icon, title, subtitle, found) {
    const status =
      found === undefined
        ? ''
        : `<div class="st ${found ? '' : 'bad'}"><i></i>${found ? 'Đã tìm thấy file' : 'Không tìm thấy file'}</div>`;
    return `<div class="mh"><div class="tile tile-primary">${icon(20)}</div>
      <div class="grow"><div class="name" style="font-size:14px">${esc(title)}</div>
      ${[].concat(subtitle).map((line) => `<div class="${found === undefined ? 'code' : 'path'}">${esc(line)}</div>`).join('')}${status}</div></div>`;
  }

  function fileMenu(trigger, s) {
    const fileName = s.path.split('/').pop();
    const subtitle = s.displayName === fileName ? [s.dir] : [fileName, s.dir];
    openMenu(trigger, headerHTML(ICON.file, s.displayName, subtitle, s.exists), [
      { group: 'Chạy', label: 'Chạy với tham số…', desc: 'Chạy một lần hoặc lưu lại để dùng sau', icon: ICON.play, style: 'hot', run: () => paramDialog(s, -1) },
      { group: 'Quản lý', label: 'Đổi tên hiển thị', desc: 'Không đổi tên file trên ổ đĩa', icon: ICON.pencil, run: () => renameDialog(s) },
      { label: 'Sao chép đường dẫn', desc: 'Đường dẫn đầy đủ vào clipboard', icon: ICON.copy, run: () => copy(s.path, 'Đã sao chép đường dẫn') },
      { label: 'Mở để sửa', desc: 'Dùng ứng dụng mặc định cho .sh', icon: ICON.external, run: () => api().OpenInEditor(s.path).catch((e) => toast(String(e))) },
      { label: 'Xóa khỏi danh sách', desc: 'File gốc trên máy không bị xóa', icon: ICON.trash, style: 'bad', divider: true, run: async () => setScripts(await api().RemoveScript(s.path)) },
    ]);
  }

  function runMenu(trigger, s, j) {
    const r = s.runs[j];
    openMenu(trigger, headerHTML(ICON.play, r.name, r.params), [
      { group: 'Chỉnh sửa', label: 'Sửa tên và tham số', desc: 'Cập nhật lần chạy đã lưu', icon: ICON.pencil, style: 'hot', run: () => paramDialog(s, j) },
      { label: 'Sao chép tham số', desc: 'Dán vào Terminal hoặc lần chạy khác', icon: ICON.copy, run: () => copy(r.params, 'Đã sao chép tham số') },
      { label: 'Xóa lần chạy này', desc: `File ${s.displayName} vẫn được giữ lại`, icon: ICON.trash, style: 'bad', divider: true, run: async () => setScripts(await api().DeleteRun(s.path, j)) },
    ]);
  }

  menu.addEventListener('click', (e) => {
    const btn = e.target.closest('.mi');
    if (!btn) return;
    const it = menu._items[Number(btn.dataset.idx)];
    // Đóng menu trước rồi mới chạy hành động, để menu không còn trên màn hình khi có hộp thoại.
    closeMenu(false);
    Promise.resolve()
      .then(it.run)
      .catch((err) => toast(String(err)));
  });

  menu.addEventListener('keydown', (e) => {
    const items = [...menu.querySelectorAll('.mi')];
    const i = items.indexOf(document.activeElement);
    if (e.key === 'ArrowDown') items[(i + 1) % items.length].focus();
    else if (e.key === 'ArrowUp') items[(i - 1 + items.length) % items.length].focus();
    else if (e.key === 'Home') items[0].focus();
    else if (e.key === 'End') items[items.length - 1].focus();
    else if (e.key === 'Escape') closeMenu(true);
    else if (e.key === 'Tab') closeMenu(true);
    else return;
    e.preventDefault();
  });

  document.addEventListener('pointerdown', (e) => {
    if (!menu.hidden && !menu.contains(e.target) && !(menuTrigger && menuTrigger.contains(e.target))) closeMenu(false);
  });
  window.addEventListener('blur', () => closeMenu(false));
  window.addEventListener('resize', () => closeMenu(false));
  $('list').addEventListener('scroll', () => closeMenu(false), { passive: true });

  // ---------- Hộp thoại ----------

  function wireDialog(dlg) {
    dlg.addEventListener('click', (e) => {
      if (e.target.closest('[data-close]')) dlg.close();
      else if (e.target === dlg) dlg.close(); // bấm ra ngoài hộp thoại
    });
  }
  const paramDlg = $('paramDlg');
  const renameDlg = $('renameDlg');
  [paramDlg, renameDlg].forEach(wireDialog);
  document.querySelectorAll('[data-close].ico').forEach((b) => (b.innerHTML = ICON.close(18)));
  paramDlg.querySelector('.tile').innerHTML = ICON.play(20);

  function showDialog(dlg, focusEl) {
    dlg.showModal();
    focusEl.focus();
    focusEl.select();
  }

  function paramDialog(s, idx) {
    const editing = idx >= 0;
    $('paramTitle').textContent = editing ? 'Sửa lần chạy' : 'Chạy với tham số';
    $('paramFile').textContent = `${s.displayName} · ${s.dir}`;
    $('paramName').value = editing ? s.runs[idx].name : '';
    $('paramValue').value = editing ? s.runs[idx].params : '';
    $('paramError').hidden = true;
    $('paramRun').hidden = false;
    $('paramRun').disabled = !s.exists;
    $('paramChips').innerHTML = s.runs
      .filter((r, j) => j !== idx)
      .map((r) => `<button class="chip-btn" type="button" data-fill="${esc(r.params)}">${esc(r.params)}</button>`)
      .join('');
    paramDlg._ctx = { s, idx, editing };
    showDialog(paramDlg, $('paramValue'));
  }

  $('paramChips').addEventListener('click', (e) => {
    const b = e.target.closest('[data-fill]');
    if (b) {
      $('paramValue').value = b.dataset.fill;
      $('paramValue').focus();
    }
  });

  $('paramRun').addEventListener('click', () => {
    const { s } = paramDlg._ctx;
    paramDlg.close();
    api().Run(s.path, $('paramValue').value);
  });

  async function saveParam() {
    const { s, idx } = paramDlg._ctx;
    try {
      setScripts(await api().SaveRun(s.path, idx, $('paramName').value, $('paramValue').value));
      paramDlg.close();
    } catch (e) {
      const err = $('paramError');
      err.textContent = String(e && e.message ? e.message : e);
      err.hidden = false;
      $('paramValue').focus();
    }
  }
  $('paramSave').addEventListener('click', saveParam);
  // Enter: đang sửa thì lưu, còn không thì chạy một lần.
  paramDlg.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter' || e.target.closest('button')) return;
    e.preventDefault();
    (paramDlg._ctx.editing ? $('paramSave') : $('paramRun')).click();
  });

  function renameDialog(s) {
    $('renameValue').value = s.displayName;
    $('renameValue').placeholder = s.path.split('/').pop();
    renameDlg._ctx = { s };
    showDialog(renameDlg, $('renameValue'));
  }
  $('renameForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const { s } = renameDlg._ctx;
    renameDlg.close();
    setScripts(await api().RenameScript(s.path, $('renameValue').value));
  });

  // ---------- Output ----------

  const log = $('log');
  const chip = $('chip');

  function atBottom() {
    return log.scrollHeight - log.scrollTop - log.clientHeight < 40;
  }

  // appendOutput nối đoạn mới vào output. Chỉ tự cuộn xuống khi người dùng đang ở cuối, để họ cuộn lên đọc được.
  function appendOutput(kind, text) {
    const stick = atBottom();
    const last = log.lastElementChild;
    if (last && last.dataset.k === kind) {
      last.lastChild.nodeValue += text;
    } else {
      const span = document.createElement('span');
      span.dataset.k = kind;
      if (kind !== 'out') span.className = 'k-' + kind;
      span.appendChild(document.createTextNode(text));
      log.appendChild(span);
    }
    if (stick) log.scrollTop = log.scrollHeight;
  }

  function updateChip() {
    if (state.running.size) {
      chip.className = 'chip live';
      chip.innerHTML = '<span class="dot"></span>Đang chạy';
      chip.hidden = false;
    } else if (state.lastResult) {
      const ok = state.lastResult === 'ok';
      chip.className = 'chip' + (ok ? '' : ' fail');
      chip.innerHTML = `<span class="dot"></span>${ok ? 'Hoàn tất · exit 0' : 'Có lỗi'}`;
      chip.hidden = false;
    } else {
      chip.hidden = true;
    }
  }

  $('copyOut').innerHTML = ICON.copy(16);
  $('clearOut').innerHTML = ICON.trash(16);
  $('copyOut').addEventListener('click', () => copy(log.textContent, 'Đã sao chép output'));
  $('clearOut').addEventListener('click', () => {
    log.textContent = '';
    state.lastResult = null;
    updateChip();
  });

  // ---------- Thanh chia ----------

  const split = $('split');
  const root = document.documentElement;
  const clampOut = (h) => Math.max(120, Math.min(h, window.innerHeight - 260));
  const setOut = (h) => root.style.setProperty('--out-h', clampOut(h) + 'px');
  try {
    const saved = Number(localStorage.getItem('outH'));
    if (saved) setOut(saved);
  } catch (_) {}
  const saveOut = () => {
    try {
      localStorage.setItem('outH', String(parseInt(getComputedStyle($('out')).height, 10)));
    } catch (_) {}
  };
  split.addEventListener('pointerdown', (e) => {
    split.setPointerCapture(e.pointerId);
    split.classList.add('drag');
  });
  split.addEventListener('pointermove', (e) => {
    if (split.classList.contains('drag')) setOut(window.innerHeight - e.clientY);
  });
  split.addEventListener('pointerup', () => {
    split.classList.remove('drag');
    saveOut();
  });
  split.addEventListener('keydown', (e) => {
    const step = e.shiftKey ? 48 : 16;
    const cur = parseInt(getComputedStyle($('out')).height, 10);
    if (e.key === 'ArrowUp') setOut(cur + step);
    else if (e.key === 'ArrowDown') setOut(cur - step);
    else return;
    e.preventDefault();
    saveOut();
  });

  // ---------- Khác ----------

  let toastTimer;
  function toast(msg) {
    const t = $('toast');
    t.textContent = msg;
    t.classList.add('show');
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => t.classList.remove('show'), 1800);
  }

  async function copy(text, msg) {
    try {
      await api().CopyText(text);
      toast(msg);
    } catch (e) {
      toast('Không sao chép được');
    }
  }

  // ---------- Sự kiện ----------

  $('add').addEventListener('click', addFile);

  $('list').addEventListener('click', (e) => {
    const btn = e.target.closest('[data-action]');
    if (!btn) return;
    const s = state.scripts && state.scripts[Number(btn.dataset.i)];
    switch (btn.dataset.action) {
      case 'run':
        api().Run(btn.dataset.path, btn.dataset.params);
        break;
      case 'menu-file':
        // Bấm lại đúng nút đang mở menu thì đóng menu.
        if (menuTrigger === btn) closeMenu(false);
        else fileMenu(btn, s);
        break;
      case 'menu-run':
        if (menuTrigger === btn) closeMenu(false);
        else runMenu(btn, s, Number(btn.dataset.j));
        break;
      case 'add':
        addFile();
        break;
      case 'retry':
        load();
        break;
    }
  });

  function onRuntimeEvents() {
    const rt = window.runtime;
    rt.EventsOn('output', (ev) => appendOutput(ev.kind, ev.text));
    rt.EventsOn('clear', () => (log.textContent = ''));
    rt.EventsOn('run', (ev) => {
      if (ev.running) {
        state.running.add(ev.key);
      } else {
        state.running.delete(ev.key);
        state.lastResult = ev.ok ? 'ok' : 'fail';
      }
      updateChip();
      render();
    });
  }

  // Wails chèn window.go và window.runtime sau khi trang tải xong.
  function start() {
    if (api() && window.runtime) {
      onRuntimeEvents();
      load();
    } else {
      setTimeout(start, 30);
    }
  }
  render();
  start();
})();
