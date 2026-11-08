import { startBrowserDownload, DocumentFileError, type DownloadedFile } from './document-files';

interface CopyDestination {
  createWritable(): Promise<{ write(data: Blob): Promise<void>; close(): Promise<void>; abort(): Promise<void> }>;
}
type PickerWindow = Window & {
  showSaveFilePicker?: (options: { suggestedName: string; types: Array<{description: string;accept: Record<string,string[]>}> }) => Promise<CopyDestination>;
};

// Call during the explicit button gesture, before network work can consume
// user activation. No handle or future overwrite permission is retained.
export async function chooseDOCXCopyDestination(name: string): Promise<CopyDestination | undefined> {
  const owner = (window.frameElement?.hasAttribute('data-document-editor') ? window.parent : window) as PickerWindow;
  if (!owner.showSaveFilePicker) return;
  return owner.showSaveFilePicker({
    suggestedName: name,
    types: [{ description: 'DOCX', accept: {'application/vnd.openxmlformats-officedocument.wordprocessingml.document':['.docx']} }],
  });
}

export async function writeDOCXCopy(file: DownloadedFile, destination?: CopyDestination): Promise<'saved' | 'download'> {
  if (!destination) { startBrowserDownload(file); return 'download'; }
  const writer = await destination.createWritable();
  try {
    await writer.write(file.blob);
    await writer.close();
  } catch (error) {
    try { await writer.abort(); } catch { /* Keep the original write failure. */ }
    throw error;
  }
  return 'saved';
}

export async function exportDOCXCopy(input: {sessionId:string;baseRevision:number;operations:unknown[];name:string}): Promise<DownloadedFile> {
  const response = await fetch(`/api/docx/export?document=${encodeURIComponent(input.sessionId)}`, {
    method:'POST', credentials:'same-origin', cache:'no-store', redirect:'error',
    headers:{'Content-Type':'application/json'}, body:JSON.stringify(input),
  });
  if (!response.ok) throw new DocumentFileError('DOCX copy export failed',response.status);
  const mime='application/vnd.openxmlformats-officedocument.wordprocessingml.document';
  if (response.headers.get('X-Document-ID')!==input.sessionId ||
      response.headers.get('X-Document-Revision')!==String(input.baseRevision) ||
      response.headers.get('Content-Type')?.split(';')[0]!==mime) {
    throw new DocumentFileError('DOCX copy revision or content type mismatch',409);
  }
  const blob=await response.blob();
  if(!blob.size||blob.size>32*1024*1024)throw new DocumentFileError('DOCX copy size exceeds limit',413);
  return {blob,name:input.name};
}
