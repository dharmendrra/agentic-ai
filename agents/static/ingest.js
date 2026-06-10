/**
 * OmniRAG Ingest page — uploads a PDF to /api/ingest (same origin as the agent).
 */
'use strict';

const drop = document.getElementById('drop');
const fileInput = document.getElementById('file');
const filename = document.getElementById('filename');
const titleInput = document.getElementById('title');
const submit = document.getElementById('submit');
const status = document.getElementById('status');

function log(line) {
  status.classList.remove('hidden');
  status.textContent += (status.textContent ? '\n' : '') + line;
  status.scrollTop = status.scrollHeight;
}

function showFile() {
  const f = fileInput.files[0];
  filename.textContent = f ? f.name : '';
}

drop.addEventListener('click', () => fileInput.click());
fileInput.addEventListener('change', showFile);

['dragenter', 'dragover'].forEach(ev =>
  drop.addEventListener(ev, e => { e.preventDefault(); drop.classList.add('border-emerald-500'); }));
['dragleave', 'drop'].forEach(ev =>
  drop.addEventListener(ev, e => { e.preventDefault(); drop.classList.remove('border-emerald-500'); }));
drop.addEventListener('drop', e => {
  const f = e.dataTransfer.files && e.dataTransfer.files[0];
  if (!f) return;
  if (f.type !== 'application/pdf' && !f.name.toLowerCase().endsWith('.pdf')) {
    log('Only PDF files are supported.');
    return;
  }
  fileInput.files = e.dataTransfer.files;
  showFile();
});

submit.addEventListener('click', async () => {
  const f = fileInput.files[0];
  status.textContent = '';
  if (!f) { log('Choose or drop a PDF first.'); return; }
  submit.disabled = true;
  log('> uploading ' + f.name + ' …');
  const fd = new FormData();
  fd.append('file', f);
  if (titleInput.value.trim()) fd.append('book_title', titleInput.value.trim());
  try {
    const r = await fetch('/api/ingest', { method: 'POST', body: fd });
    const text = await r.text();
    if (!r.ok) { log('ERROR: ' + text); return; }
    const j = JSON.parse(text);
    log('OK: ' + j.message);
    log(`book="${j.book_title}"  pages=${j.pages}  chunks=${j.chunks}  upserted=${j.upserted}`);
  } catch (e) { log('ERROR: ' + e); }
  finally { submit.disabled = false; }
});
