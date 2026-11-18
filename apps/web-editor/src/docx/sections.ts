import type { SectionFormat,SectionInfo,HeaderFooterInfo } from './contracts';

export const defaultHeaderFooterDistance = 720;

export function previewSection(original:SectionInfo,format:SectionFormat):SectionInfo {
 const page={...original.page};
 for(const key of ['width','height','top','right','bottom','left'] as const)if(format[key]!==undefined)page[key]=format[key]!;
 return {
  ...original,page,gutter:format.gutter??original.gutter,
  headerDistance:format.headerDistance??original.headerDistance,
  footerDistance:format.footerDistance??original.footerDistance,
  pageNumberStart:format.pageNumberStart===-1?undefined:format.pageNumberStart??original.pageNumberStart,
  pageNumberFormat:format.pageNumberFormat??original.pageNumberFormat,
  firstPage:format.firstPage??original.firstPage,
  orientation:format.orientation??original.orientation,
  columns:format.columns??original.columns,
  columnSpace:format.columnSpace??original.columnSpace,
  breakType:format.breakType??original.breakType,
 };
}
export function sectionPaper(section:SectionInfo):SectionInfo['page'] {
 const page=section.page;
 const safeGutter=Math.min(section.gutter??0,Math.max(0,page.width-page.left-page.right-1200));
 return safeGutter?{...page,left:page.left+safeGutter}:page;
}

export function validateSection(section:SectionInfo):string {
 const {page,columns,columnSpace}=section;
 const bodyWidth=page.width-page.left-page.right-(section.gutter??0);
 if(bodyWidth<1200||page.height-page.top-page.bottom<1200)return '여백을 줄여 본문 영역을 확보하세요.';
 if(bodyWidth-(columns-1)*columnSpace<columns*240)return '단 수나 간격을 줄여 본문 폭을 확보하세요.';
 return '';
}


export function previewSections(original:SectionInfo[],updates:Map<string,SectionFormat>):SectionInfo[] {
 const previous=new Map<string,HeaderFooterInfo>();
 return original.map(section=>{
  const patch=updates.get(section.id)??{},next=previewSection(section,patch);
  next.headerFooters=section.headerFooters?.map(slot=>{
   const key=slot.kind+':'+slot.variant,update=patch.headerFooters?.find(item=>item.kind===slot.kind&&item.variant===slot.variant);
   let info={...slot};
   if(update&&!update.linkPrevious)info={...info,text:update.text??'',pageNumber:update.pageNumber??slot.pageNumber,totalPages:update.totalPages??slot.totalPages,linked:false,sourceSectionId:section.id,editable:true,truncated:false};
   else if(update?.linkPrevious||slot.linked) {
    const inherited=previous.get(key);info={...info,text:'',linked:true,pageNumber:inherited?.pageNumber,totalPages:inherited?.totalPages,sourceSectionId:inherited?.sourceSectionId,editable:inherited?.editable??true,truncated:inherited?.truncated};
   }
   previous.set(key,info);return info;
  });
  return next;
 });
}
