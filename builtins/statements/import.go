package statements

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/nikolalohinski/gonja/v2/nodes"
	"github.com/nikolalohinski/gonja/v2/parser"
	"github.com/nikolalohinski/gonja/v2/tokens"
)

type ImportStmt struct {
	location           *tokens.Token
	filenameExpression nodes.Expression
	as                 string
	withContext        bool
}

func (stmt *ImportStmt) Position() *tokens.Token {
	return stmt.location
}

func (stmt *ImportStmt) String() string {
	t := stmt.Position()
	return fmt.Sprintf("ImportStmt(Line=%d Col=%d)", t.Line, t.Col)
}

func (stmt *ImportStmt) Execute(r *exec.Renderer, tag *nodes.StatementBlock) error {

	filenameValue := r.Eval(stmt.filenameExpression)
	if filenameValue.IsError() {
		return errors.Wrap(filenameValue, `Unable to evaluate filename`)
	}

	filename := filenameValue.String()
	loader, err := r.Loader.Inherit(filename)
	if err != nil {
		return fmt.Errorf("failed to inherit loader from '%s': %s", filename, r.Loader)
	}

	template, err := exec.NewTemplate(filename, r.Config, loader, r.Environment)
	if err != nil {
		return fmt.Errorf("unable to load template '%s': %s", filename, err)
	}

	macros := map[string]exec.Macro{}
	for name, macro := range template.Macros() {
		fn, err := exec.MacroNodeToFunc(macro, r)
		if err != nil {
			return errors.Wrapf(err, `Unable to import macro '%s'`, name)
		}
		macros[name] = fn
	}
	r.Environment.Context.Set(stmt.as, macros)

	return nil
}

type FromImportStmt struct {
	location           *tokens.Token
	FilenameExpression nodes.Expression
	WithContext        bool
	Template           *nodes.Template
	As                 map[string]string
	Macros             map[string]*nodes.Macro // alias/name -> macro instance
}

func (stmt *FromImportStmt) Position() *tokens.Token {
	return stmt.location
}

func (stmt *FromImportStmt) String() string {
	t := stmt.Position()
	return fmt.Sprintf("FromImportStmt(Line=%d Col=%d)", t.Line, t.Col)
}

func (stmt *FromImportStmt) Execute(r *exec.Renderer, tag *nodes.StatementBlock) error {

	filenameValue := r.Eval(stmt.FilenameExpression)
	if filenameValue.IsError() {
		return errors.Wrap(filenameValue, `Unable to evaluate filename`)
	}

	filename := filenameValue.String()
	loader, err := r.Loader.Inherit(filename)
	if err != nil {
		return fmt.Errorf("failed to inherit loader from '%s': %s", filename, r.Loader)
	}

	template, err := exec.NewTemplate(filename, r.Config, loader, r.Environment)
	if err != nil {
		return fmt.Errorf("unable to load template '%s': %s", filename, err)
	}

	imported := template.Macros()
	for alias, name := range stmt.As {
		node := imported[name]
		fn, err := exec.MacroNodeToFunc(node, r)
		if err != nil {
			return errors.Wrapf(err, `Unable to import macro '%s'`, name)
		}
		r.Environment.Context.Set(alias, fn)
	}
	return nil
}

func importParser(p *parser.Parser, args *parser.Parser) (nodes.Statement, error) {
	stmt := &ImportStmt{
		location: p.Current(),
		// Macros:   map[string]*nodes.Macro{},
	}

	if args.End() {
		return nil, args.Error("You must at least specify one macro to import.", nil)
	}

	expression, err := args.ParseExpression()
	if err != nil {
		return nil, err
	}
	stmt.filenameExpression = expression
	if args.MatchName("as") == nil {
		return nil, args.Error(`Expected "as" keyword`, args.Current())
	}

	alias := args.Match(tokens.Name)
	if alias == nil {
		return nil, args.Error("Expected macro alias name (identifier)", args.Current())
	}
	stmt.as = alias.Val

	if tok := args.MatchName("with", "without"); tok != nil {
		if args.MatchName("context") != nil {
			stmt.withContext = tok.Val == "with"
		} else {
			args.Stream().Backup()
		}
	}
	return stmt, nil
}

func fromParser(p *parser.Parser, args *parser.Parser) (nodes.Statement, error) {
	stmt := &FromImportStmt{
		location: p.Current(),
		As:       map[string]string{},
	}

	if args.End() {
		return nil, args.Error("You must at least specify one macro to import.", nil)
	}

	filename, err := args.ParseExpression()
	if err != nil {
		return nil, err
	}
	stmt.FilenameExpression = filename

	if args.MatchName("import") == nil {
		return nil, args.Error("Expected import keyword", args.Current())
	}

	for !args.End() {
		name := args.Match(tokens.Name)
		if name == nil {
			return nil, args.Error("Expected macro name (identifier).", args.Current())
		}

		// asName := macroNameToken.Val
		if args.MatchName("as") != nil {
			alias := args.Match(tokens.Name)
			if alias == nil {
				return nil, args.Error("Expected macro alias name (identifier).", nil)
			}
			// asName = aliasToken.Val
			stmt.As[alias.Val] = name.Val
		} else {
			stmt.As[name.Val] = name.Val
		}

		if tok := args.MatchName("with", "without"); tok != nil {
			if args.MatchName("context") != nil {
				stmt.WithContext = tok.Val == "with"
				break
			} else {
				args.Stream().Backup()
			}
		}

		if args.End() {
			break
		}

		if args.Match(tokens.Comma) == nil {
			return nil, args.Error("Expected ','.", nil)
		}
	}

	return stmt, nil
}

func init() {
	All.Register("import", importParser)
	All.Register("from", fromParser)
}
