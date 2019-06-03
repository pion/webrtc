package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

const cmdUsage = `
Usage : gnaliasdocs [options] <file_name>
Options:
	-pkg (mandatory) the location of the aliased package
	-GOOS (optional) GOOS used to filter which files are considered when collection comments
	-GOARCH (optional) GOARCH used to filter which files are considered when collection comments
    -build-tags (optional) additional build tags used to filter which files are considered when collection comments`

func main() {
	var (
		pkg          string
		goos         string
		goarch       string
		buildTagsStr string
	)

	flag.StringVar(&pkg, "pkg", "", "the location of the aliased package")
	flag.StringVar(&goos, "GOOS", "", "GOOS used to filter which files are considered when collection comments")
	flag.StringVar(&goarch, "GOARCH", "", "GOARCH used to filter which files are considered when collection comments")
	flag.StringVar(&buildTagsStr, "build-tags", "", "space separated build tags")
	flag.Parse()

	if pkg == "" {
		fmt.Println(cmdUsage)
		return
	}
	var buildTags []string
	if buildTagsStr != "" {
		buildTags = strings.Split(buildTagsStr, " ")
	}

	args := flag.Args()
	if len(args) != 1 {
		fmt.Println(cmdUsage)
		return
	}

	f := args[0]
	if !path.IsAbs(pkg) {
		pkg = path.Join(path.Dir(f), pkg)
	}
	aliasedComments, err := getAliasedComments(pkg, goos, goarch, buildTags)
	if err != nil {
		fmt.Printf("Error collecting aliased comments: %v", err)
	}

	for _, arg := range args {
		files, err := filepath.Glob(arg)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
			return
		}
		for _, f := range files {
			generate(f, aliasedComments)
		}
	}
}

func generate(fileName string, aliasedComments map[string]string) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, fileName, nil, parser.ParseComments)
	if err != nil {
		fmt.Printf("Error parsing file: %v", err)
		return
	}

	// need this because Inspect will skip comments in the root node (eg: the generate comment)
	origignalComments := make(map[token.Pos]*ast.CommentGroup, len(node.Comments))
	for _, c := range node.Comments {
		origignalComments[c.Pos()] = c
	}

	comments := make([]*ast.CommentGroup, 0)

	ast.Inspect(node, func(n ast.Node) bool {
		switch t := n.(type) {
		case *ast.CommentGroup:
			delete(origignalComments, t.Pos())
			comments = append(comments, t)

		case *ast.ValueSpec:
			if len(t.Values) != len(t.Names) {
				return true
			}

			cg := getCommentGroup(t.Names, t.Values, t.Pos(), aliasedComments)
			if cg != nil {
				if t.Doc != nil {
					for _, l := range t.Doc.List {
						delete(origignalComments, l.Slash)
					}
				}
				t.Doc = cg
			}

		case *ast.TypeSpec:
			cg := getCommentGroup([]*ast.Ident{t.Name}, []ast.Expr{t.Type}, t.Pos(), aliasedComments)
			if cg != nil {
				if t.Doc != nil {
					for _, l := range t.Doc.List {
						delete(origignalComments, l.Slash)
					}
				}
				t.Doc = cg
			}
		default:
		}

		return true
	})
	for _, c := range origignalComments {
		comments = append(comments, c)
		sort.Slice(comments, func(i, j int) bool {
			return comments[i].Pos() < comments[j].Pos()
		})
	}

	node.Comments = comments

	// overwrite the file with modified version of ast.
	write, err := os.Create(fileName)
	if err != nil {
		fmt.Printf("Error opening file %v", err)
		return
	}
	defer func() {
		err = write.Close()
		if err != nil {
			fmt.Println(err)
		}
	}()
	w := bufio.NewWriter(write)
	err = format.Node(w, fset, node)
	if err != nil {
		fmt.Printf("Error formating file %s", err)
		return
	}
	err = w.Flush()
	if err != nil {
		fmt.Printf("Error writing file %s", err)
		return
	}
}

func getCommentGroup(names []*ast.Ident, values []ast.Expr, pos token.Pos, aliasedComments map[string]string) *ast.CommentGroup {
	if len(names) != len(values) {
		return nil
	}

	cg := &ast.CommentGroup{
		List: make([]*ast.Comment, 0),
	}
	for i, name := range names {
		v, ok := values[i].(*ast.SelectorExpr)
		if !ok {
			continue
		}

		aliasedName := v.Sel.Name
		c, ok := aliasedComments[aliasedName]
		if !ok || c == "" {
			c = fmt.Sprintf("%s TODO: missing comment", aliasedName)
		}

		c = strings.TrimSpace(c)
		for _, l := range strings.Split(c, "\n") {
			l = strings.TrimSpace(l)

			cg.List = append(cg.List, &ast.Comment{
				Slash: pos - 1,
				Text:  "// " + strings.Replace(l, aliasedName, name.Name, -1),
			})
		}
	}

	if len(cg.List) == 0 {
		return nil
	}

	return cg
}

func getAliasedComments(dir string, goos string, goarch string, buildTags []string) (map[string]string, error) {
	files, err := filepath.Glob(dir + "/*.go")
	if err != nil {
		return nil, err
	}

	comments := make(map[string]string)

	buildCtx := build.Default
	if goos != "" {
		buildCtx.GOOS = goos
	}
	if goarch != "" {
		buildCtx.GOARCH = goarch
	}
	if len(buildTags) > 0 {
		buildCtx.BuildTags = buildTags
	}

	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}

		match, err := buildCtx.MatchFile(path.Dir(f), path.Base(f))
		if err != nil {
			return nil, fmt.Errorf("error parsing file %v", err.Error())
		}

		if !match {
			continue
		}

		fset := token.NewFileSet()
		// Parse the file given in arguments
		node, err := parser.ParseFile(fset, f, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("error parsing file %v", err.Error())
		}

		for _, c := range node.Comments {
			name := strings.SplitN(c.Text(), " ", 2)[0]
			exported := false
			for _, first := range name {
				if unicode.IsUpper(first) {
					exported = true
				}
				break
			}

			if exported {
				comments[name] = c.Text()
			}
		}

	}

	return comments, nil
}
