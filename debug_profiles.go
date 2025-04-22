package go_webserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	httpprof "net/http/pprof"
	"net/url"
	"runtime/pprof"
	"strings"
	"time"

	pprofprofile "github.com/google/pprof/profile"
)

// -----------------------------------------------------------------------------

type debugProfile struct {
	name    string
	profile *pprof.Profile
	handler HandlerFunc
}

type flameGraphNode struct {
	FunctionName    string            `json:"functionName"`
	FileName        string            `json:"fileName"`
	Line            int64             `json:"line"`
	Nanoseconds     int64             `json:"nanos"`
	SelfNanoseconds int64             `json:"selfNanos"`
	Children        []*flameGraphNode `json:"childs,omitempty"`
}

// -----------------------------------------------------------------------------

var debugProfiles []debugProfile

// -----------------------------------------------------------------------------

// ServeDebugProfiles adds the GO runtime profile handlers to a web server
func (srv *Server) ServeDebugProfiles(basePath string, middlewares ...HandlerFunc) {
	// Prepare a debug profile array if not done yet
	if debugProfiles == nil {
		for _, profile := range pprof.Profiles() {
			debugProfiles = append(debugProfiles, debugProfile{
				name:    profile.Name(),
				profile: profile,
				handler: NewHandlerFromHttpHandler(httpprof.Handler(profile.Name())),
			})
		}
		debugProfiles = append(debugProfiles, debugProfile{
			name:    "cmdline",
			handler: NewHandlerFromHttpHandlerFunc(httpprof.Cmdline),
		})
		debugProfiles = append(debugProfiles, debugProfile{
			name:    "profile",
			handler: onGenerateProfile, // NewHandlerFromHttpHandlerFunc(httpprof.Profile),
		})
		debugProfiles = append(debugProfiles, debugProfile{
			name:    "symbol",
			handler: NewHandlerFromHttpHandlerFunc(httpprof.Symbol),
		})
		debugProfiles = append(debugProfiles, debugProfile{
			name:    "trace",
			handler: NewHandlerFromHttpHandlerFunc(httpprof.Trace),
		})
	}

	// Fix slashes
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	if strings.HasSuffix(basePath, "/") {
		basePath = basePath[:len(basePath)-1]
	}

	// Add an index page
	srv.GET(basePath, onDebugProfilesIndex, middlewares...)

	// Add profile pages
	for _, p := range debugProfiles {
		srv.GET(basePath+"/"+p.name, p.handler, middlewares...)
	}
}

func onDebugProfilesIndex(req *RequestContext) error {
	var b bytes.Buffer

	// Get base url
	path := string(req.URI().Path())
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}

	req.SetResponseHeader("X-Content-Type-Options", "nosniff")
	req.SetResponseHeader("Content-Type", "text/html; charset=utf-8")

	// Write index header
	_, _ = b.WriteString(`<!doctype html>
<html>
<head>
<title>Debug profiles</title>
<style>
body {
	font-family: monospace;
}
table {
	border-collapse:collapse;
}
td {
	padding: 2px;
}
td.header {
	border-bottom: 1px solid #000;
}
td.vsep {
	border-right: 1px solid #000;
}
td.ralign {
	text-align: right;
}
</style>
</head>
<body>
<table>
	<thead>
		<td class='header vsep'>Profile</td>
		<td class='header ralign'>Count</td>
	</thead>
	<tbody>
`)

	// Write each profile on the main table
	for _, p := range debugProfiles {
		link := &url.URL{
			Path: path + p.name,
		}

		if p.profile != nil {
			link.RawQuery = "debug=1"
		} else if p.name == "profile" {
			link.RawQuery = "seconds=15"
		}
		_, _ = fmt.Fprintf(&b, `		<tr>
			<td class='vsep'><a href='%s'>%s</a>`, link, html.EscapeString(p.name))

		switch p.name {
		case "goroutine":
			link.RawQuery = "debug=2"
			_, _ = fmt.Fprintf(&b, ` (<a href='%s'>full</a>)`, link)
		}

		if p.profile != nil {
			_, _ = fmt.Fprintf(&b, `</td>
			<td class='ralign'>%d</td>
		</tr>
`, p.profile.Count())
		} else {
			_, _ = b.WriteString(`</td>
			<td></td>
		</tr>
`)
		}
	}

	// Close table and html pae
	_, _ = b.WriteString(`	</tbody>
</table>
</body>
</html>
`)

	// Write response
	_, _ = req.Write(b.Bytes())
	req.Success()

	// Done
	return nil
}

func onGenerateProfile(req *RequestContext) error {
	secs, err := req.QueryArgs().GetUint("seconds")
	if secs <= 0 || err != nil {
		secs = 30
	}

	format := req.QueryArgs().Peek("format")
	if len(format) == 0 {
		format = req.QueryArgs().Peek("fmt")
	}
	switch string(format) {
	case "":
		fallthrough
	case "binary":
		req.SetResponseHeader("Content-Type", "application/octet-stream")
		req.SetResponseHeader("Content-Disposition", `attachment; filename="profile"`)

		err = gatherCpuProfile(req, secs, req)
		if err != nil {
			req.InternalServerError(fmt.Sprintf("CPU profiling failed [err=%s]", err))
			return nil
		}

	case "json":
		var b bytes.Buffer
		var pf *pprofprofile.Profile
		var flameGraphRootNode *flameGraphNode

		err = gatherCpuProfile(req, secs, &b)
		if err != nil {
			req.InternalServerError(fmt.Sprintf("CPU profiling failed [err=%s]", err))
			return nil
		}

		// Use google-pprof to parse the profile
		pf, err = pprofprofile.Parse(&b)
		if err != nil {
			req.InternalServerError(fmt.Sprintf("Unable to parse CPU profile [err=%s]", err))
			return nil
		}

		flameGraphRootNode, err = createFlameGraph(pf)
		if err != nil {
			req.InternalServerError(fmt.Sprintf("Unable to create flame graph [err=%s]", err))
			return nil
		}

		req.WriteJSON(flameGraphRootNode)

	default:
		req.BadRequest("Unsupported format")
	}

	// Done
	return nil
}

func gatherCpuProfile(ctx context.Context, secs int, w io.Writer) error {
	err := pprof.StartCPUProfile(w)
	if err != nil {
		return err
	}
	defer pprof.StopCPUProfile()

	select {
	case <-time.After(time.Duration(secs) * time.Second):
	case <-ctx.Done():
		return ctx.Err()
	}

	// Done
	return nil
}

// Parse the pprof profile and collapse the stacks into a flame graph structure.
func createFlameGraph(profile *pprofprofile.Profile) (*flameGraphNode, error) {
	root := &flameGraphNode{
		FunctionName: "root",
	}

	cpuTimeIdx := -1
	for idx, sampleType := range profile.SampleType {
		if sampleType.Type == "cpu" && sampleType.Unit == "nanoseconds" {
			cpuTimeIdx = idx
			break
		}
	}
	if cpuTimeIdx < 0 {
		return nil, errors.New("unable to find cpu time information")
	}

	// Iterate through the profile's samples.
	for _, sample := range profile.Sample {
		root.Nanoseconds += sample.Value[cpuTimeIdx]

		node := root
		for i := len(sample.Location) - 1; i >= 0; i-- {
			var childNode *flameGraphNode

			// Locate child node
			if len(sample.Location[i].Line) == 0 {
				continue
			}
			functionName := sample.Location[i].Line[0].Function.Name

			for _, child := range node.Children {
				if child.FunctionName == functionName {
					childNode = child
					break
				}
			}
			// Create a new child node if not found
			if childNode == nil {
				childNode = &flameGraphNode{
					FunctionName: functionName,
					FileName:     sample.Location[i].Line[0].Function.Filename,
					Line:         sample.Location[i].Line[0].Line,
				}
				node.Children = append(node.Children, childNode)
			}

			childNode.Nanoseconds += sample.Value[cpuTimeIdx]
			if i == 0 {
				childNode.SelfNanoseconds += sample.Value[cpuTimeIdx]
			}
			node = childNode
		}
	}

	// Done
	return root, nil
}
