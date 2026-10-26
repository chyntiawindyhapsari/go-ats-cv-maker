package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
)

func xmlEscape(s string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	).Replace(s)
}

func makeDOCX(c CV) ([]byte, error) {
	var body strings.Builder

	addText := func(text string, bold bool, size int) {
		if strings.TrimSpace(text) == "" {
			return
		}

		b := ""
		if bold {
			b = "<w:b/>"
		}

		body.WriteString(fmt.Sprintf(
			`<w:p>
<w:pPr>
<w:spacing w:after="100"/>
</w:pPr>
<w:r>
<w:rPr>
<w:rFonts w:ascii="Arial" w:hAnsi="Arial"/>
%s
<w:sz w:val="%d"/>
<w:color w:val="34435D"/>
</w:rPr>
<w:t xml:space="preserve">%s</w:t>
</w:r>
</w:p>`,
			b,
			size,
			xmlEscape(text),
		))
	}

	addSection := func(label, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}

		body.WriteString(fmt.Sprintf(
			`<w:p>
<w:pPr>
<w:spacing w:before="220" w:after="100"/>
<w:pBdr>
<w:bottom>
<w:val="single"/>
<w:sz w:val="4"/>
<w:space w:val="4"/>
<w:color w:val="D5DDEC"/>
</w:bottom>
</w:pBdr>
</w:pPr>
<w:r>
<w:rPr>
<w:rFonts w:ascii="Arial" w:hAnsi="Arial"/>
<w:b/>
<w:sz w:val="20"/>
<w:color w:val="34435D"/>
</w:rPr>
<w:t>%s</w:t>
</w:r>
</w:p>`,
			xmlEscape(label),
		))

		for _, line := range strings.Split(value, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				addText(line, false, 19)
			}
		}
	}

	// Identitas
	addText(c.Name, true, 36)
	addText(c.Title, true, 22)

	addText(strings.Join(nonempty([]string{
		c.Email,
		c.Phone,
		c.Location,
		c.LinkedIn,
		c.Website,
	}), " | "), false, 18)

	// Foto profil
	photo, format, err := decodePhoto(c.Photo)
	if err != nil {
		return nil, err
	}

	imageExtension := ""
	imageContentType := ""

	if len(photo) > 0 {
		switch strings.ToUpper(format) {
		case "PNG":
			imageExtension = "png"
			imageContentType = "image/png"
		case "JPG", "JPEG":
			imageExtension = "jpeg"
			imageContentType = "image/jpeg"
		default:
			return nil, fmt.Errorf("Format foto tidak didukung: %s", format)
		}

		body.WriteString(`
<w:p>
<w:pPr>
<w:spacing w:after="180"/>
</w:pPr>
<w:r>
<w:drawing>
<wp:inline xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"
distT="0" distB="0" distL="0" distR="0">
<wp:extent cx="952500" cy="952500"/>
<wp:docPr id="1" name="Foto Profil"/>
<wp:cNvGraphicFramePr>
<a:graphicFrameLocks noChangeAspect="1"/>
</wp:cNvGraphicFramePr>
<a:graphic>
<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">
<pic:pic>
<pic:nvPicPr>
<pic:cNvPr id="0" name="Foto Profil"/>
<pic:cNvPicPr/>
</pic:nvPicPr>
<pic:blipFill>
<a:blip r:embed="rId2"
xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"/>
<a:stretch>
<a:fillRect/>
</a:stretch>
</pic:blipFill>
<pic:spPr>
<a:xfrm>
<a:off x="0" y="0"/>
<a:ext cx="952500" cy="952500"/>
</a:xfrm>
<a:prstGeom prst="rect">
<a:avLst/>
</a:prstGeom>
</pic:spPr>
</pic:pic>
</a:graphicData>
</a:graphic>
</wp:inline>
</w:drawing>
</w:r>
</w:p>`)
	}

	// Bagian CV dipisah satu per satu.
	sections := sectionLabels(c)

	for _, s := range sections {
		addSection(s.label, s.value)
	}

	document := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document
xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<w:body>
` + body.String() + `
<w:sectPr>
<w:pgSz w:w="11906" w:h="16838"/>
<w:pgMar w:top="850" w:right="1000" w:bottom="850" w:left="1000"/>
</w:sectPr>
</w:body>
</w:document>`

	var buf bytes.Buffer
	z := zip.NewWriter(&buf)

	types := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>`

	rootRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1"
Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
Target="word/document.xml"/>
</Relationships>`

	docRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`

	if len(photo) > 0 {
		types += fmt.Sprintf(
			`<Default Extension="%s" ContentType="%s"/>`,
			imageExtension,
			imageContentType,
		)

		docRels += fmt.Sprintf(
			`<Relationship Id="rId2"
Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"
Target="media/profile.%s"/>`,
			imageExtension,
		)
	}

	types += `</Types>`
	docRels += `</Relationships>`

	parts := map[string]string{
		"[Content_Types].xml":          types,
		"_rels/.rels":                  rootRels,
		"word/document.xml":            document,
		"word/_rels/document.xml.rels": docRels,
	}

	for name, data := range parts {
		f, err := z.Create(name)
		if err != nil {
			return nil, err
		}

		if _, err := f.Write([]byte(data)); err != nil {
			return nil, err
		}
	}

	if len(photo) > 0 {
		f, err := z.Create("word/media/profile." + imageExtension)
		if err != nil {
			return nil, err
		}

		if _, err := f.Write(photo); err != nil {
			return nil, err
		}
	}

	if err := z.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
