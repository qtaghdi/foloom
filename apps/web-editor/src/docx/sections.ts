import type { SectionFormat,SectionInfo } from './contracts';

export function previewSection(original:SectionInfo,format:SectionFormat):SectionInfo {
 const page={...original.page};
 for(const key of ['width','height','top','right','bottom','left'] as const)if(format[key]!==undefined)page[key]=format[key]!;
 return {...original,page,orientation:format.orientation??original.orientation,columns:format.columns??original.columns,columnSpace:format.columnSpace??original.columnSpace,breakType:format.breakType??original.breakType};
}
export function validateSection(section:SectionInfo):string {
 const {page,columns,columnSpace}=section;
 if(page.width-page.left-page.right<1200||page.height-page.top-page.bottom<1200)return '여백을 줄여 본문 영역을 확보하세요.';
 if(page.width-page.left-page.right-(columns-1)*columnSpace<columns*240)return '단 수나 간격을 줄여 본문 폭을 확보하세요.';
 return '';
}
