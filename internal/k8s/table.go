package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

// Column is a single table column header.
type Column struct {
	Name string
	// Priority > 0 marks "wide" columns hidden by default (matches kubectl).
	Priority int
}

// Row is a single rendered table row plus the metadata needed to act on it.
type Row struct {
	Cells     []string
	Name      string
	Namespace string
	Dimmed    bool
}

// Table is the rendered, server-side-printed view of a resource list.
type Table struct {
	Columns []Column
	Rows    []Row
}

// tableAccept negotiates server-side table printing. Do not include plain JSON:
// a normal list decodes into an empty metav1.Table and hides data.
const tableAccept = "application/json;as=Table;v=v1;g=meta.k8s.io," +
	"application/json;as=Table;v=v1beta1;g=meta.k8s.io"

// restClientFor builds a REST client bound to a specific group/version so we
// can request the Table representation for an arbitrary resource.
func (c *Client) restClientFor(gv schema.GroupVersion) (rest.Interface, error) {
	cfg := rest.CopyConfig(c.restConfig)
	cfg.GroupVersion = &gv
	if gv.Group == "" {
		cfg.APIPath = "/api"
	} else {
		cfg.APIPath = "/apis"
	}
	cfg.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	return rest.RESTClientFor(cfg)
}

// ListTable fetches a resource list as a server-printed table. When namespace
// is "" and the resource is namespaced, it lists across all namespaces and
// prepends a NAMESPACE column. A non-empty fieldSelector narrows the list
// server-side (e.g. spec.nodeName=x for the pods on one node).
func (c *Client) ListTable(ctx context.Context, res ResourceInfo, namespace, fieldSelector string) (*Table, error) {
	gv := schema.GroupVersion{Group: res.Group, Version: res.Version}
	rc, err := c.restClientFor(gv)
	if err != nil {
		return nil, err
	}

	req := rc.Get().
		Resource(res.Resource).
		SetHeader("Accept", tableAccept).
		Param("includeObject", "Metadata")
	if res.Namespaced && namespace != "" {
		req = req.Namespace(namespace)
	}
	if fieldSelector != "" {
		req = req.Param("fieldSelector", fieldSelector)
	}

	raw, err := req.Do(ctx).Raw()
	if err != nil {
		return nil, err
	}

	mt, err := decodeTable(raw, res.Resource)
	if err != nil {
		return nil, err
	}

	showNS := res.Namespaced && namespace == ""
	return convertTable(mt, res, showNS), nil
}

func decodeTable(raw []byte, resource string) (*metav1.Table, error) {
	var mt metav1.Table
	if err := json.Unmarshal(raw, &mt); err != nil {
		return nil, fmt.Errorf("decode table for %s: %w", resource, err)
	}
	if mt.Kind != "" && mt.Kind != "Table" {
		return nil, fmt.Errorf("decode table for %s: server returned %s", resource, mt.Kind)
	}
	return &mt, nil
}

func convertTable(mt *metav1.Table, res ResourceInfo, showNS bool) *Table {
	t := &Table{}

	if showNS {
		t.Columns = append(t.Columns, Column{Name: "NAMESPACE"})
	}
	for _, cd := range mt.ColumnDefinitions {
		t.Columns = append(t.Columns, Column{Name: cd.Name, Priority: int(cd.Priority)})
	}

	statusCol := -1
	if res.IsPod() {
		for i, c := range t.Columns {
			if strings.EqualFold(c.Name, "Status") {
				statusCol = i
				break
			}
		}
	}
	for i := range mt.Rows {
		r := &mt.Rows[i]
		var name, ns string
		if len(r.Object.Raw) > 0 {
			var pom metav1.PartialObjectMetadata
			if json.Unmarshal(r.Object.Raw, &pom) == nil {
				name = pom.Name
				ns = pom.Namespace
			}
		}

		cells := make([]string, 0, len(r.Cells)+1)
		if showNS {
			cells = append(cells, ns)
		}
		for _, c := range r.Cells {
			cells = append(cells, cellToString(c))
		}

		row := Row{Cells: cells, Name: name, Namespace: ns}
		if row.Name == "" && len(r.Cells) > 0 {
			row.Name = cellToString(r.Cells[0])
		}
		if statusCol >= 0 && statusCol < len(row.Cells) &&
			strings.EqualFold(strings.TrimSpace(row.Cells[statusCol]), "Completed") {
			row.Dimmed = true
		}
		t.Rows = append(t.Rows, row)
	}

	return t
}

func cellToString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(t, 10)
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}
