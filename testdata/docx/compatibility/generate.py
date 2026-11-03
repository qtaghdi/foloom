"""Generate deterministic synthetic DOCX fixtures using only Python's standard library."""
import base64
import hashlib
import json
from pathlib import Path
import zipfile

ROOT = Path(__file__).resolve().parent
WORD_NS = 'http://schemas.openxmlformats.org/wordprocessingml/2006/main'
REL_NS = 'http://schemas.openxmlformats.org/package/2006/relationships'
OFFICE_NS = 'http://schemas.openxmlformats.org/officeDocument/2006/relationships'
CONTENT_TYPES = '''<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Default Extension="png" ContentType="image/png"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/header1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/></Types>'''
RELATIONSHIPS = f'<Relationships xmlns="{REL_NS}"><Relationship Id="rId1" Type="{OFFICE_NS}/officeDocument" Target="word/document.xml"/></Relationships>'
BODY = '''<w:p><w:r><w:rPr><w:b/></w:rPr><w:t xml:space="preserve"> 한글 </w:t></w:r><w:r><w:t>repeat</w:t></w:r><w:r><w:rPr><w:i/></w:rPr><w:t>repeat</w:t></w:r></w:p><w:tbl><w:tblPr><w:tblW w:w="0" w:type="auto"/></w:tblPr><w:tblGrid><w:gridCol w:w="2400"/><w:gridCol w:w="2400"/></w:tblGrid><w:tr><w:tc><w:p><w:r><w:t>A</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>B</w:t></w:r></w:p></w:tc></w:tr></w:tbl>'''
PROTECTED = '''<w:p><w:hyperlink r:id="link1"><w:r><w:t>외부 링크</w:t></w:r></w:hyperlink></w:p><w:p><w:fldSimple w:instr="PAGE"><w:r><w:t>1</w:t></w:r></w:fldSimple></w:p><w:p><w:ins w:id="1" w:author="Synthetic" w:date="2026-11-03T00:00:00Z"><w:r><w:t>변경 추적</w:t></w:r></w:ins></w:p><w:sectPr><w:headerReference w:type="default" r:id="header1"/></w:sectPr>'''
manifest = []
for name, protected in [('generated-basic.docx', False), ('generated-preservation.docx', True)]:
    parts = {
        '[Content_Types].xml': CONTENT_TYPES.encode(),
        '_rels/.rels': RELATIONSHIPS.encode(),
        'word/document.xml': (f'<w:document xmlns:w="{WORD_NS}" xmlns:r="{OFFICE_NS}"><w:body>' + BODY + (PROTECTED if protected else '<w:sectPr/>') + '</w:body></w:document>').encode(),
        'word/header1.xml': f'<w:hdr xmlns:w="{WORD_NS}"><w:p><w:r><w:t>머리글 원본</w:t></w:r></w:p></w:hdr>'.encode(),
        'word/media/image1.png': base64.b64decode('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jH/UAAAAASUVORK5CYII='),
        'customXml/item1.xml': b'<custom xmlns="urn:synthetic" preserve="yes">untouched</custom>',
    }
    if protected:
        parts['word/_rels/document.xml.rels'] = f'<Relationships xmlns="{REL_NS}"><Relationship Id="header1" Type="{OFFICE_NS}/header" Target="header1.xml"/><Relationship Id="link1" Type="{OFFICE_NS}/hyperlink" Target="https://example.invalid/never-fetch" TargetMode="External"/><Relationship Id="image1" Type="{OFFICE_NS}/image" Target="media/image1.png"/></Relationships>'.encode()
    destination = ROOT / name
    with zipfile.ZipFile(destination, 'w') as archive:
        for key, data in parts.items():
            info = zipfile.ZipInfo(key, date_time=(2026, 11, 3, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            info.external_attr = 0o100600 << 16
            archive.writestr(info, data)
    manifest.append({
        'file': name, 'sha256': hashlib.sha256(destination.read_bytes()).hexdigest(),
        'producer': 'generate.py v1, Python standard-library zipfile',
        'format': 'OOXML transitional DOCX', 'date': '2026-11-03',
        'rights': 'Repository-owned synthetic content, no user documents',
        'features': ['Korean and Latin text', 'mixed run formatting', 'repeated text', 'whitespace', 'simple table', 'unmodified custom XML and image part'] + (['header relationship', 'external hyperlink', 'field', 'tracked insertion'] if protected else []),
        'nativeWord': 'not verified', 'nativeLibreOffice': 'not verified',
    })
(ROOT / 'manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n')
