package main

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"

	"cdr.dev/slog"
	"cdr.dev/slog/sloggers/sloghuman"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/td0m/zima"
	"oss.terrastruct.com/d2/d2format"
	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2layouts/d2dagrelayout"
	"oss.terrastruct.com/d2/d2lib"
	"oss.terrastruct.com/d2/d2oracle"
	"oss.terrastruct.com/d2/d2renderers/d2svg"
	"oss.terrastruct.com/d2/d2themes/d2themescatalog"
	d2log "oss.terrastruct.com/d2/lib/log"
	"oss.terrastruct.com/d2/lib/textmeasure"
)

func p[T any](t T) *T {
	return &t
}

func toD2ID(s zima.Set) string {
	tid := s.Type + ":" + s.ID
	if s.IsSingleton() {
		return tid
	}
	return tid + "#" + s.Relation
}

func toLink(set zima.Set) string {
	q := url.Values{}
	q.Set("type", set.Type)
	q.Set("id", set.ID)
	q.Set("direction", "children")

	return "[" + toD2ID(set) + "](/foo?" + q.Encode() + ")"
}

type setdef struct {
	id  int
	Set zima.Set
}

func buildSvg(ctx context.Context, tuples []zima.Tuple, backward bool) ([]byte, error) {
	// From one.go
	ruler, _ := textmeasure.NewRuler()
	layoutResolver := func(engine string) (d2graph.LayoutGraph, error) {
		return d2dagrelayout.DefaultLayout, nil
	}
	compileOpts := &d2lib.CompileOptions{
		LayoutResolver: layoutResolver,
		Ruler:          ruler,
	}
	_, graph, _ := d2lib.Compile(ctx, "", compileOpts, nil)

	direction := "down"
	// if backward {
	// 	direction = "up"
	// }
	graph, _ = d2oracle.Set(graph, nil, "direction", nil, p(direction))

	all := map[zima.Set]setdef{}
	for _, t := range tuples {
		if t.Parent.Relation != "" {
			all[t.Parent] = setdef{Set: zima.Set{ID: t.Parent.ID, Type: t.Parent.Type}}
		}
		all[t.Child] = setdef{Set: t.Child}
	}

	i := 0
	for k, v := range all {
		v.id = i
		all[k] = v

		graph, _, _ = d2oracle.Create(graph, nil, fmt.Sprintf(`s%d`, i))
		graph, _ = d2oracle.Set(graph, nil, fmt.Sprintf(`s%d`, i), nil, p(""))

		// graph, _ = d2oracle.Set(graph, nil, fmt.Sprintf(`s%d.style.fill`, i), nil, p("red"))
		graph, _ = d2oracle.Set(graph, nil, fmt.Sprintf(`s%d.style.fill`, i), nil, nil)

		graph, _ = d2oracle.Set(graph, nil, fmt.Sprintf(`s%d.content`, i), p("md"), p(toLink(v.Set)))
		i++
	}

	for _, t := range tuples {
		graph, _ = d2oracle.Set(graph, nil, fmt.Sprintf(`s%d -> s%d`, all[t.Parent].id, all[t.Child].id), nil, p(t.Parent.Relation))
	}

	// // Create a shape with the ID, "meow"
	// graph, _, _ = d2oracle.Create(graph, nil, "meow")
	// // Style the shape green
	// color := "green"
	// graph, err := d2oracle.Set(graph, nil, "meow.style.fill", nil, &color)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed: %w", err)
	// }
	//
	// graph, err = d2oracle.Set(graph, nil, "a -> b", nil, nil)
	// fmt.Println(err)
	// // Create a shape with the ID, "cat"
	// graph, _, _ = d2oracle.Create(graph, nil, "cat")
	// // Move the shape "meow" inside the container "cat"
	// graph, _ = d2oracle.Move(graph, nil, "meow", "cat.meow", false)
	// graph, _ = d2oracle.Create
	// Prints formatted D2 script
	text := d2format.Format(graph.AST)
	fmt.Println(text)

	// // Render
	// pad := int64(5)
	// renderOpts := &d2svg.RenderOpts{
	// 	Pad:     &pad,
	// 	ThemeID: &d2themescatalog.GrapeSoda.ID,
	// }
	// out, _ := d2svg.Render(text, renderOpts)
	return toSvg(ctx, text)
}

func toSvg(ctx context.Context, d string) ([]byte, error) {
	ruler, _ := textmeasure.NewRuler()
	layoutResolver := func(engine string) (d2graph.LayoutGraph, error) {
		return d2dagrelayout.DefaultLayout, nil
	}

	pad := int64(5)
	renderOpts := &d2svg.RenderOpts{
		Pad:     &pad,
		ThemeID: &d2themescatalog.GrapeSoda.ID,
	}
	compileOpts := &d2lib.CompileOptions{
		LayoutResolver: layoutResolver,
		Ruler:          ruler,
	}
	diagram, _, _ := d2lib.Compile(ctx, d, compileOpts, renderOpts)
	out, _ := d2svg.Render(diagram, renderOpts)

	return out, nil
}

func main() {
	ctx := context.Background()
	conn, err := pgxpool.New(ctx, "")
	if err != nil {
		panic(err)
	}
	srv := zima.NewServer(conn)

	tmpl, err := template.New("templates/*.tmpl").ParseFiles("templates/foo.tmpl")
	if err != nil {
		panic(err)
	}

	r := chi.NewRouter()
	r.With(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(d2log.With(r.Context(), slog.Make(sloghuman.Sink(io.Discard))))
			next.ServeHTTP(w, r)
		})
	})

	r.Get("/foo", func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(d2log.With(r.Context(), slog.Make(sloghuman.Sink(io.Discard))))
		q := r.URL.Query()

		err := srv.Write(ctx, []zima.Tuple{
			zima.NewTuple(S("collection", "ufo", "member"), S("user", "dom", "")),
		}, nil)
		fmt.Println(err)

		typ, id, direction := q.Get("type"), q.Get("id"), q.Get("direction")
		var tuples []zima.Tuple
		if direction == "children" {
			res, err := srv.ListChildren(ctx, zima.ListChildrenRequest{Type: typ, ID: id})
			if err != nil {
				panic(err)
			}
			tuples = make([]zima.Tuple, len(res.Items))
			for i, other := range res.Items {
				tuples[i] = zima.NewTuple(zima.Set{Type: typ, ID: id, Relation: other.Relation}, other.Set)
			}
		} else {
			res, err := srv.ListParents(ctx, zima.ListParentsRequest{Type: typ, ID: id})
			if err != nil {
				panic(err)
			}
			tuples = make([]zima.Tuple, len(res.Items))
			for i, other := range res.Items {
				tuples[i] = zima.NewTuple(other, zima.Set{ID: id, Type: typ})
				fmt.Println(tuples[i])
			}
		}

		svg, err := buildSvg(r.Context(), tuples, direction != "children")
		if err != nil {
			panic(err)
		}

		w.Header().Add("content-type", "text/html")
		tmpl.ExecuteTemplate(w, "page", map[string]any{
			"svg": template.HTML(svg),
			"direction": direction,
		})
	})

	fmt.Println("listening..")
	http.ListenAndServe(":8000", r)
}

func S(t, i, r string) zima.Set {
	return try(zima.NewSet(t, i, r))
}

func try[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}

	return t
}
