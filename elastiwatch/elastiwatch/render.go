package elastiwatch

import (
	"bytes"
	"html/template"
)

type renderSection struct {
	Heading string
	Chunks  []chunk
}

type htmlRenderer struct {
}

type htmlRenderRow struct {
	Chunk chunk
	Color string
}

func (h htmlRenderer) rowColor(c chunk) string {
	switch c.Severity {
	case "DEBUG":
		return "gray"
	case "INFO":
		return "green"
	case "WARNING":
		return "orange"
	case "ERROR":
		return "red"
	case "CRITICAL":
		return "purple"
	}
	return "black"
}

type htmlRenderSection struct {
	Heading string
	Rows    []htmlRenderRow
}

func (h htmlRenderer) makeObj(sections []renderSection) (res []htmlRenderSection) {
	for _, s := range sections {
		var htmlSection htmlRenderSection
		htmlSection.Heading = s.Heading
		for _, c := range s.Chunks {
			htmlSection.Rows = append(htmlSection.Rows, htmlRenderRow{
				Chunk: c,
				Color: h.rowColor(c),
			})
		}
		res = append(res, htmlSection)
	}
	return res
}

func (h htmlRenderer) Render(sections []renderSection) (string, error) {
	const templateText = `
	<html>
		<head>
		<style>
			table {
				margin: 25px;
			}
			td {
				padding-right: 20px;
			}
		</style>
		</head>
		<body>
		{{ range $key, $value := . }}
			<h3>{{$value.Heading}}</h3>
			<table>
			{{ range $rkey, $rvalue := $value.Rows }}
				<tr style="color:{{$rvalue.Color}}">
					<td>{{ $rvalue.Chunk.Time }}</td>
					<td>{{ $rvalue.Chunk.Count }}</td>
					<td>{{ $rvalue.Chunk.Severity }}</td>
					<td>{{ $rvalue.Chunk.Message }}</td>
				</tr>
			{{ end }}
			</table>
		{{ end }}
		</body>
	</html>`

	t := template.New("t")
	t, err := t.Parse(templateText)
	if err != nil {
		return "", err
	}

	obj := h.makeObj(sections)

	var out bytes.Buffer
	err = t.Execute(&out, obj)
	if err != nil {
		return "", err
	}

	return out.String(), nil
}
