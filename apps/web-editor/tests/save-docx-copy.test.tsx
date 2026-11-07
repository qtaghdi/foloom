import { afterEach, expect, it, vi } from 'vitest';
import { chooseDOCXCopyDestination, writeDOCXCopy } from '../src/transport/save-docx-copy';

afterEach(()=>vi.unstubAllGlobals());

it('writes only to the explicitly picked destination and closes before reporting success',async()=>{
 const write=vi.fn(async()=>{}),close=vi.fn(async()=>{}),abort=vi.fn(async()=>{});
 const destination={createWritable:vi.fn(async()=>({write,close,abort}))};
 const picker=vi.fn(async()=>destination);
 Object.defineProperty(window,'showSaveFilePicker',{configurable:true,value:picker});
 try {
  const picked=await chooseDOCXCopyDestination('new.docx');
  expect(picker).toHaveBeenCalledWith(expect.objectContaining({suggestedName:'new.docx'}));
  const blob=new Blob(['checked bytes']);
  expect(await writeDOCXCopy({name:'new.docx',blob},picked)).toBe('saved');
  expect(write).toHaveBeenCalledWith(blob);
  expect(close).toHaveBeenCalledOnce();
  expect(abort).not.toHaveBeenCalled();
 } finally {delete (window as Window&{showSaveFilePicker?:unknown}).showSaveFilePicker;}
});

it('aborts a failed file write and never reports it as saved',async()=>{
 const failure=new Error('disk full'),abort=vi.fn(async()=>{}),close=vi.fn(async()=>{});
 const destination={createWritable:async()=>({write:async()=>{throw failure;},close,abort})};
 await expect(writeDOCXCopy({name:'copy.docx',blob:new Blob(['bytes'])},destination)).rejects.toBe(failure);
 expect(abort).toHaveBeenCalledOnce();
 expect(close).not.toHaveBeenCalled();
});
