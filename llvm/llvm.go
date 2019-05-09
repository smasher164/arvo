package llvm

import (
	"fmt"
	"strconv"

	"github.com/smasher164/arvo/ast"
	"github.com/smasher164/arvo/scan"

	"github.com/smasher164/arvo/types"

	"llvm.org/llvm/bindings/go/llvm"
)

type Generator struct {
	Config  types.Config
	builder llvm.Builder
	mod     llvm.Module
	builtin map[string]llvm.Value
	m       map[ast.Node]llvm.Value
	fnstk   []llvm.Value
	bb      map[llvm.Value][]llvm.BasicBlock
	ifposq  []int
}

func (g *Generator) pushfn(fn llvm.Value) {
	g.fnstk = append(g.fnstk, fn)
}

func (g *Generator) topfn() llvm.Value {
	return g.fnstk[len(g.fnstk)-1]
}

func (g *Generator) popfn() (fn llvm.Value) {
	fn, g.fnstk = g.fnstk[len(g.fnstk)-1], g.fnstk[:len(g.fnstk)-1]
	return
}

func (g *Generator) set(n ast.Node, v llvm.Value) {
	switch n := n.(type) {
	case *ast.Ident:
		if n != nil {
			if n.Obj != nil {
				if n.Obj.Kind == ast.Fun {
					if f, _ := n.Obj.Decl.(*ast.FunDef); f != nil {
						// maybe an issue with anonymous functions?
						g.m[f.Name] = v
					}
				} else if n.Obj.Kind == ast.Var {
					if pr, _ := n.Obj.Decl.(*ast.Param); pr != nil {
						g.m[pr.Name] = v
					} else if as, _ := n.Obj.Decl.(*ast.AssignStmt); as != nil {
						for _, e := range as.Lhs {
							if id, _ := e.(*ast.Ident); id != nil {
								if id.Name.Lit == n.Name.Lit {
									g.m[id] = v
									break
								}
							}
						}
					}
				}
			} else {
				g.m[n] = v
			}
		}
		g.m[n] = v
	default:
		g.m[n] = v
	}
}

func (g *Generator) get(n ast.Node) llvm.Value {
	switch n := n.(type) {
	case *ast.Ident:
		if n != nil {
			if n.Obj != nil {
				if n.Obj.Kind == ast.Fun {
					if f, _ := n.Obj.Decl.(*ast.FunDef); f != nil {
						return g.m[f.Name]
					}
				} else if n.Obj.Kind == ast.Var {
					if pr, _ := n.Obj.Decl.(*ast.Param); pr != nil {
						return g.m[pr.Name]
					} else if as, _ := n.Obj.Decl.(*ast.AssignStmt); as != nil {
						for _, e := range as.Lhs {
							if id, _ := e.(*ast.Ident); id != nil {
								if id.Name.Lit == n.Name.Lit {
									return g.m[id]
								}
							}
						}
					}
				}
			} else {
				return g.m[n]
			}
		}
		return g.m[n]
	default:
		return g.m[n]
	}
}

func (g *Generator) pre(n ast.Node) bool {
	switch t := n.(type) {
	case *ast.IfStmt:
		_ = t
		// Thread position information for basic blocks on the way down.
		ip := len(g.bb[g.topfn()])
		g.ifposq = append(g.ifposq, ip)
	case *ast.FunDef:
		switch t.Name.Name.Lit {
		case "printf", "exit":
			return false
		}
		// sig, ok := g.Config.Get(t).(types.Signature)
		// if !ok {
		//     panic("function type is not Signature")
		// }
		// sig
		// main := llvm.FunctionType(llvm.Int32Type(), []llvm.Type{}, false)
		// fn := llvm.AddFunction(g.mod, "main", main)
		// g.pushfn(mainfn)
		// block := llvm.AddBasicBlock(mainfn, "entry")
		// g.bb[mainfn] = append(g.bb[mainfn], block)
		// g.builder.SetInsertPointAtEnd(block)
	case *ast.BlockStmt:
		tfn := g.topfn()
		entry := g.bb[tfn][len(g.bb[tfn])-1]
		g.builder.SetInsertPointAtEnd(entry)
		bb := llvm.AddBasicBlock(tfn, "")
		g.bb[tfn] = append(g.bb[tfn], bb)
		// g.nop()
		g.builder.SetInsertPointAtEnd(bb)
	}
	return true
}

func (g *Generator) nop() {
	g.builder.CreateAdd(llvm.ConstInt(llvm.Int64Type(), 0, false), llvm.ConstInt(llvm.Int64Type(), 0, false), "")
}

// assignment will associate ident with value
// use of ident in other statements/exprs will reference it

func (g *Generator) post(n ast.Node) bool {
	switch t := n.(type) {
	case *ast.Ident:
		if t.Name.Lit == "true" {
			v := g.builder.CreateAlloca(llvm.Int64Type(), "")
			g.builder.CreateStore(llvm.ConstInt(llvm.Int64Type(), 1, false), v)
			g.set(n, g.builder.CreateLoad(v, ""))
		} else if t.Name.Lit == "false" {
			v := g.builder.CreateAlloca(llvm.Int64Type(), "")
			g.builder.CreateStore(llvm.ConstInt(llvm.Int64Type(), 0, false), v)
			g.set(n, g.builder.CreateLoad(v, ""))
		}
	case *ast.BasicLit:
		if types.Match(t.Value.Type, scan.Int) {
			i, err := strconv.Atoi(t.Value.Lit)
			if err != nil {
				panic(err)
			}
			v := g.builder.CreateAlloca(llvm.Int64Type(), "")
			g.builder.CreateStore(llvm.ConstInt(llvm.Int64Type(), uint64(i), true), v)
			g.set(n, g.builder.CreateLoad(v, ""))
		} else if types.Match(t.Value.Type, scan.String) {
			val, err := strconv.Unquote("\"" + t.Value.Lit[1:len(t.Value.Lit)-1] + "\"")
			if err != nil {
				panic(err)
			}
			sptr := g.builder.CreateAlloca(llvm.PointerType(llvm.Int8Type(), 0), "")
			s := g.builder.CreateCall(g.builtin["alloc_string"], []llvm.Value{}, "")
			g.builder.CreateCall(g.builtin["init_c_str"], []llvm.Value{s, g.builder.CreateGlobalStringPtr(val, "")}, "")
			g.builder.CreateStore(s, sptr)
			g.set(n, g.builder.CreateLoad(sptr, ""))
		}
	case *ast.ParenExpr:
		g.set(n, g.get(t.X))
	case *ast.CallExpr:
		if id, _ := t.Fun.(*ast.Ident); id != nil {
			switch id.Name.Lit {
			case "exit":
				g.builder.CreateCall(g.builtin["exit"], []llvm.Value{
					g.builder.CreateIntCast(g.get(t.Args[0]), llvm.Int32Type(), ""),
				}, "")
			case "printf":
				values := []llvm.Value{g.builder.CreateCall(g.builtin["c_str"], []llvm.Value{g.get(t.Args[0])}, "")}
				for i := 1; i < len(t.Args); i++ {
					arg := g.get(t.Args[i])
					values = append(values, g.builder.CreateCall(g.builtin["c_str"], []llvm.Value{arg}, ""))
				}
				g.builder.CreateCall(g.builtin["printf"], values, "")
			}
		}
	case *ast.UnaryExpr:
		switch t.Op.Type {
		case scan.Add:
			g.set(n, g.get(t.X))
		case scan.Sub:
			diff := g.builder.CreateAlloca(llvm.Int64Type(), "")
			g.builder.CreateStore(g.builder.CreateSub(llvm.ConstInt(llvm.Int64Type(), 0, false), g.get(t.X), ""), diff)
			g.set(n, g.builder.CreateLoad(diff, ""))
		case scan.Not:
			// logical negation
			res := g.builder.CreateAlloca(llvm.Int64Type(), "")
			cmp := g.builder.CreateICmp(llvm.IntNE, g.get(t.X), llvm.ConstInt(llvm.Int64Type(), 0, false), "")
			xor := g.builder.CreateXor(cmp, llvm.ConstInt(llvm.Int64Type(), 1, false), "")
			zext := g.builder.CreateZExt(xor, llvm.Int32Type(), "")
			sext := g.builder.CreateSExt(zext, llvm.Int64Type(), "")
			g.builder.CreateStore(sext, res)
			g.set(n, g.builder.CreateLoad(res, ""))
		case scan.Xor:
			// bitwise complement
			res := g.builder.CreateAlloca(llvm.Int64Type(), "")
			g.builder.CreateStore(g.builder.CreateNeg(g.get(t.X), ""), res)
			g.set(n, g.builder.CreateLoad(res, ""))
		}
	case *ast.BinaryExpr:
		typX := g.Config.Get(t.X)
		switch t.Op.Type {
		case scan.Lor:
			tfn := g.topfn()
			entry := g.bb[tfn][len(g.bb[tfn])-1]
			falseL := llvm.AddBasicBlock(tfn, "")
			merge := llvm.AddBasicBlock(tfn, "")
			g.bb[tfn] = append(g.bb[tfn], falseL, merge)
			g.builder.SetInsertPointAtEnd(entry)
			z := g.builder.CreateAlloca(llvm.Int64Type(), "")
			cmp1 := g.builder.CreateICmp(llvm.IntNE, g.get(t.X), llvm.ConstInt(llvm.Int64Type(), 0, false), "")
			g.builder.CreateCondBr(cmp1, merge, falseL)
			g.builder.SetInsertPointAtEnd(falseL)
			cmp2 := g.builder.CreateICmp(llvm.IntNE, g.get(t.Y), llvm.ConstInt(llvm.Int64Type(), 0, false), "")
			zext := g.builder.CreateZExt(cmp2, llvm.Int32Type(), "")
			sext := g.builder.CreateSExt(zext, llvm.Int64Type(), "")
			g.builder.CreateBr(merge)
			g.builder.SetInsertPointAtEnd(merge)
			phi := g.builder.CreatePHI(llvm.Int64Type(), "")
			phi.AddIncoming([]llvm.Value{llvm.ConstInt(llvm.Int64Type(), 1, false), sext}, []llvm.BasicBlock{entry, falseL})
			g.builder.CreateStore(phi, z)
			g.set(n, g.builder.CreateLoad(z, ""))
		case scan.Land:
			tfn := g.topfn()
			entry := g.bb[tfn][len(g.bb[tfn])-1]
			trueL := llvm.AddBasicBlock(tfn, "")
			merge := llvm.AddBasicBlock(tfn, "")
			g.bb[tfn] = append(g.bb[tfn], trueL, merge)
			g.builder.SetInsertPointAtEnd(entry)
			z := g.builder.CreateAlloca(llvm.Int64Type(), "")
			cmp1 := g.builder.CreateICmp(llvm.IntNE, g.get(t.X), llvm.ConstInt(llvm.Int64Type(), 0, false), "")
			g.builder.CreateCondBr(cmp1, trueL, merge)
			g.builder.SetInsertPointAtEnd(trueL)
			cmp2 := g.builder.CreateICmp(llvm.IntNE, g.get(t.Y), llvm.ConstInt(llvm.Int64Type(), 0, false), "")
			zext := g.builder.CreateZExt(cmp2, llvm.Int32Type(), "")
			sext := g.builder.CreateSExt(zext, llvm.Int64Type(), "")
			g.builder.CreateBr(merge)
			g.builder.SetInsertPointAtEnd(merge)
			phi := g.builder.CreatePHI(llvm.Int64Type(), "")
			phi.AddIncoming([]llvm.Value{llvm.ConstInt(llvm.Int64Type(), 0, false), sext}, []llvm.BasicBlock{entry, trueL})
			g.builder.CreateStore(phi, z)
			g.set(n, g.builder.CreateLoad(z, ""))

		// rel_op. TODO(composite literal comparison)
		case scan.Eql:
			if types.Match(typX, types.Bool) || types.Match(typX, types.Num) {
				res := g.builder.CreateAlloca(llvm.Int64Type(), "")
				cmp := g.builder.CreateICmp(llvm.IntEQ, g.get(t.X), g.get(t.Y), "")
				zext := g.builder.CreateZExt(cmp, llvm.Int32Type(), "")
				sext := g.builder.CreateSExt(zext, llvm.Int64Type(), "")
				g.builder.CreateStore(sext, res)
				g.set(n, g.builder.CreateLoad(res, ""))
			} else if types.Match(typX, types.String) {
				res := g.builder.CreateAlloca(llvm.Int64Type(), "")
				v := g.builder.CreateCall(g.builtin["compare_strings"], []llvm.Value{g.get(t.X), g.get(t.Y)}, "")
				cmp := g.builder.CreateICmp(llvm.IntEQ, v, llvm.ConstInt(llvm.Int32Type(), 0, false), "")
				zext := g.builder.CreateZExt(cmp, llvm.Int32Type(), "")
				sext := g.builder.CreateSExt(zext, llvm.Int64Type(), "")
				g.builder.CreateStore(sext, res)
				g.set(n, g.builder.CreateLoad(res, ""))
			}
		case scan.Neq:
			if types.Match(typX, types.Bool) || types.Match(typX, types.Num) {
				res := g.builder.CreateAlloca(llvm.Int64Type(), "")
				cmp := g.builder.CreateICmp(llvm.IntNE, g.get(t.X), g.get(t.Y), "")
				zext := g.builder.CreateZExt(cmp, llvm.Int32Type(), "")
				sext := g.builder.CreateSExt(zext, llvm.Int64Type(), "")
				g.builder.CreateStore(sext, res)
				g.set(n, g.builder.CreateLoad(res, ""))
			} else if types.Match(typX, types.String) {
				res := g.builder.CreateAlloca(llvm.Int64Type(), "")
				v := g.builder.CreateCall(g.builtin["compare_strings"], []llvm.Value{g.get(t.X), g.get(t.Y)}, "")
				cmp := g.builder.CreateICmp(llvm.IntNE, v, llvm.ConstInt(llvm.Int32Type(), 0, false), "")
				zext := g.builder.CreateZExt(cmp, llvm.Int32Type(), "")
				sext := g.builder.CreateSExt(zext, llvm.Int64Type(), "")
				g.builder.CreateStore(sext, res)
				g.set(n, g.builder.CreateLoad(res, ""))
			}
		case scan.Lss:
			if types.Match(typX, types.Bool) || types.Match(typX, types.Num) {
				res := g.builder.CreateAlloca(llvm.Int64Type(), "")
				cmp := g.builder.CreateICmp(llvm.IntSLT, g.get(t.X), g.get(t.Y), "")
				zext := g.builder.CreateZExt(cmp, llvm.Int32Type(), "")
				sext := g.builder.CreateSExt(zext, llvm.Int64Type(), "")
				g.builder.CreateStore(sext, res)
				g.set(n, g.builder.CreateLoad(res, ""))
			} else if types.Match(typX, types.String) {
				res := g.builder.CreateAlloca(llvm.Int64Type(), "")
				v := g.builder.CreateCall(g.builtin["compare_strings"], []llvm.Value{g.get(t.X), g.get(t.Y)}, "")
				cmp := g.builder.CreateICmp(llvm.IntSLT, v, llvm.ConstInt(llvm.Int32Type(), 0, false), "")
				zext := g.builder.CreateZExt(cmp, llvm.Int32Type(), "")
				sext := g.builder.CreateSExt(zext, llvm.Int64Type(), "")
				g.builder.CreateStore(sext, res)
				g.set(n, g.builder.CreateLoad(res, ""))
			}
		case scan.Leq:
			if types.Match(typX, types.Bool) || types.Match(typX, types.Num) {
				res := g.builder.CreateAlloca(llvm.Int64Type(), "")
				cmp := g.builder.CreateICmp(llvm.IntSLE, g.get(t.X), g.get(t.Y), "")
				zext := g.builder.CreateZExt(cmp, llvm.Int32Type(), "")
				sext := g.builder.CreateSExt(zext, llvm.Int64Type(), "")
				g.builder.CreateStore(sext, res)
				g.set(n, g.builder.CreateLoad(res, ""))
			} else if types.Match(typX, types.String) {
				res := g.builder.CreateAlloca(llvm.Int64Type(), "")
				v := g.builder.CreateCall(g.builtin["compare_strings"], []llvm.Value{g.get(t.X), g.get(t.Y)}, "")
				cmp := g.builder.CreateICmp(llvm.IntSLE, v, llvm.ConstInt(llvm.Int32Type(), 0, false), "")
				zext := g.builder.CreateZExt(cmp, llvm.Int32Type(), "")
				sext := g.builder.CreateSExt(zext, llvm.Int64Type(), "")
				g.builder.CreateStore(sext, res)
				g.set(n, g.builder.CreateLoad(res, ""))
			}
		case scan.Gtr:
			if types.Match(typX, types.Bool) || types.Match(typX, types.Num) {
				res := g.builder.CreateAlloca(llvm.Int64Type(), "")
				cmp := g.builder.CreateICmp(llvm.IntSGT, g.get(t.X), g.get(t.Y), "")
				zext := g.builder.CreateZExt(cmp, llvm.Int32Type(), "")
				sext := g.builder.CreateSExt(zext, llvm.Int64Type(), "")
				g.builder.CreateStore(sext, res)
				g.set(n, g.builder.CreateLoad(res, ""))
			} else if types.Match(typX, types.String) {
				res := g.builder.CreateAlloca(llvm.Int64Type(), "")
				v := g.builder.CreateCall(g.builtin["compare_strings"], []llvm.Value{g.get(t.X), g.get(t.Y)}, "")
				cmp := g.builder.CreateICmp(llvm.IntSGT, v, llvm.ConstInt(llvm.Int32Type(), 0, false), "")
				zext := g.builder.CreateZExt(cmp, llvm.Int32Type(), "")
				sext := g.builder.CreateSExt(zext, llvm.Int64Type(), "")
				g.builder.CreateStore(sext, res)
				g.set(n, g.builder.CreateLoad(res, ""))
			}
		case scan.Geq:
			if types.Match(typX, types.Bool) || types.Match(typX, types.Num) {
				res := g.builder.CreateAlloca(llvm.Int64Type(), "")
				cmp := g.builder.CreateICmp(llvm.IntSGE, g.get(t.X), g.get(t.Y), "")
				zext := g.builder.CreateZExt(cmp, llvm.Int32Type(), "")
				sext := g.builder.CreateSExt(zext, llvm.Int64Type(), "")
				g.builder.CreateStore(sext, res)
				g.set(n, g.builder.CreateLoad(res, ""))
			} else if types.Match(typX, types.String) {
				res := g.builder.CreateAlloca(llvm.Int64Type(), "")
				v := g.builder.CreateCall(g.builtin["compare_strings"], []llvm.Value{g.get(t.X), g.get(t.Y)}, "")
				cmp := g.builder.CreateICmp(llvm.IntSGE, v, llvm.ConstInt(llvm.Int32Type(), 0, false), "")
				zext := g.builder.CreateZExt(cmp, llvm.Int32Type(), "")
				sext := g.builder.CreateSExt(zext, llvm.Int64Type(), "")
				g.builder.CreateStore(sext, res)
				g.set(n, g.builder.CreateLoad(res, ""))
			}

		// add_op
		case scan.Add:
			if types.Match(typX, types.Bool) || types.Match(typX, types.Num) {
				sum := g.builder.CreateAlloca(llvm.Int64Type(), "")
				g.builder.CreateStore(g.builder.CreateAdd(g.get(t.X), g.get(t.Y), ""), sum)
				g.set(n, g.builder.CreateLoad(sum, ""))
			} else if types.Match(typX, types.String) {
				sptr := g.builder.CreateAlloca(llvm.PointerType(llvm.Int8Type(), 0), "")
				s := g.builder.CreateCall(g.builtin["concat_strings"], []llvm.Value{g.get(t.X), g.get(t.Y)}, "")
				g.builder.CreateStore(s, sptr)
				g.set(n, g.builder.CreateLoad(sptr, ""))
			}
		case scan.Sub:
			diff := g.builder.CreateAlloca(llvm.Int64Type(), "")
			g.builder.CreateStore(g.builder.CreateSub(g.get(t.X), g.get(t.Y), ""), diff)
			g.set(n, g.builder.CreateLoad(diff, ""))
		case scan.Or:
			res := g.builder.CreateAlloca(llvm.Int64Type(), "")
			g.builder.CreateStore(g.builder.CreateOr(g.get(t.X), g.get(t.Y), ""), res)
			g.set(n, g.builder.CreateLoad(res, ""))
		case scan.Xor:
			res := g.builder.CreateAlloca(llvm.Int64Type(), "")
			g.builder.CreateStore(g.builder.CreateXor(g.get(t.X), g.get(t.Y), ""), res)
			g.set(n, g.builder.CreateLoad(res, ""))

		// mul_op
		case scan.Mul:
			res := g.builder.CreateAlloca(llvm.Int64Type(), "")
			g.builder.CreateStore(g.builder.CreateMul(g.get(t.X), g.get(t.Y), ""), res)
			g.set(n, g.builder.CreateLoad(res, ""))
		case scan.Quo:
			res := g.builder.CreateAlloca(llvm.Int64Type(), "")
			g.builder.CreateStore(g.builder.CreateSDiv(g.get(t.X), g.get(t.Y), ""), res)
			g.set(n, g.builder.CreateLoad(res, ""))
		case scan.Rem:
			res := g.builder.CreateAlloca(llvm.Int64Type(), "")
			g.builder.CreateStore(g.builder.CreateSRem(g.get(t.X), g.get(t.Y), ""), res)
			g.set(n, g.builder.CreateLoad(res, ""))
		case scan.Shl:
			res := g.builder.CreateAlloca(llvm.Int64Type(), "")
			g.builder.CreateStore(g.builder.CreateShl(g.get(t.X), g.get(t.Y), ""), res)
			g.set(n, g.builder.CreateLoad(res, ""))
		case scan.Shr:
			res := g.builder.CreateAlloca(llvm.Int64Type(), "")
			g.builder.CreateStore(g.builder.CreateAShr(g.get(t.X), g.get(t.Y), ""), res)
			g.set(n, g.builder.CreateLoad(res, ""))
		case scan.And:
			res := g.builder.CreateAlloca(llvm.Int64Type(), "")
			g.builder.CreateStore(g.builder.CreateAnd(g.get(t.X), g.get(t.Y), ""), res)
			g.set(n, g.builder.CreateLoad(res, ""))
		case scan.AndNot:
			res := g.builder.CreateAlloca(llvm.Int64Type(), "")
			ny := g.builder.CreateNeg(g.get(t.Y), "")
			g.builder.CreateStore(g.builder.CreateAnd(g.get(t.X), ny, ""), res)
			g.set(n, g.builder.CreateLoad(res, ""))
		}
	case *ast.IncDecStmt:
		if t.Tok.Type == scan.Inc {
			res := g.builder.CreateAlloca(llvm.Int64Type(), "")
			g.builder.CreateStore(g.builder.CreateAdd(g.get(t.X), llvm.ConstInt(llvm.Int64Type(), 1, false), ""), res)
			g.set(t.X, g.builder.CreateLoad(res, ""))
		} else if t.Tok.Type == scan.Dec {
			res := g.builder.CreateAlloca(llvm.Int64Type(), "")
			g.builder.CreateStore(g.builder.CreateSub(g.get(t.X), llvm.ConstInt(llvm.Int64Type(), 1, false), ""), res)
			g.set(t.X, g.builder.CreateLoad(res, ""))
		}
	case *ast.AssignStmt:
		for i, l := range t.Lhs {
			typL := g.Config.Get(l)
			r := t.Rhs[i]
			if types.Match(typL, types.Bool) || types.Match(typL, types.Num) {
				switch t.Tok.Type {
				case scan.In:
				case scan.Assign:
					as := g.builder.CreateAlloca(llvm.Int64Type(), "")
					g.builder.CreateStore(g.get(r), as)
					g.set(l, g.builder.CreateLoad(as, ""))
				case scan.AddAssign:
					if !g.get(l).IsNil() {
						sum := g.builder.CreateAlloca(llvm.Int64Type(), "")
						g.builder.CreateStore(g.builder.CreateAdd(g.get(l), g.get(r), ""), sum)
						g.set(l, g.builder.CreateLoad(sum, ""))
					}
				case scan.SubAssign:
					if !g.get(l).IsNil() {
						diff := g.builder.CreateAlloca(llvm.Int64Type(), "")
						g.builder.CreateStore(g.builder.CreateSub(g.get(l), g.get(r), ""), diff)
						g.set(l, g.builder.CreateLoad(diff, ""))
					}
				case scan.MulAssign:
					if !g.get(l).IsNil() {
						res := g.builder.CreateAlloca(llvm.Int64Type(), "")
						g.builder.CreateStore(g.builder.CreateMul(g.get(l), g.get(r), ""), res)
						g.set(l, g.builder.CreateLoad(res, ""))
					}
				case scan.QuoAssign:
					if !g.get(l).IsNil() {
						res := g.builder.CreateAlloca(llvm.Int64Type(), "")
						g.builder.CreateStore(g.builder.CreateSDiv(g.get(l), g.get(r), ""), res)
						g.set(l, g.builder.CreateLoad(res, ""))
					}
				case scan.RemAssign:
					if !g.get(l).IsNil() {
						res := g.builder.CreateAlloca(llvm.Int64Type(), "")
						g.builder.CreateStore(g.builder.CreateSRem(g.get(l), g.get(r), ""), res)
						g.set(l, g.builder.CreateLoad(res, ""))
					}
				case scan.AndAssign:
					if !g.get(l).IsNil() {
						res := g.builder.CreateAlloca(llvm.Int64Type(), "")
						g.builder.CreateStore(g.builder.CreateAnd(g.get(l), g.get(r), ""), res)
						g.set(l, g.builder.CreateLoad(res, ""))
					}
				case scan.OrAssign:
					if !g.get(l).IsNil() {
						res := g.builder.CreateAlloca(llvm.Int64Type(), "")
						g.builder.CreateStore(g.builder.CreateOr(g.get(l), g.get(r), ""), res)
						g.set(l, g.builder.CreateLoad(res, ""))
					}
				case scan.XorAssign:
					if !g.get(l).IsNil() {
						res := g.builder.CreateAlloca(llvm.Int64Type(), "")
						g.builder.CreateStore(g.builder.CreateXor(g.get(l), g.get(r), ""), res)
						g.set(l, g.builder.CreateLoad(res, ""))
					}
				case scan.ShlAssign:
					if !g.get(l).IsNil() {
						res := g.builder.CreateAlloca(llvm.Int64Type(), "")
						g.builder.CreateStore(g.builder.CreateShl(g.get(l), g.get(r), ""), res)
						g.set(l, g.builder.CreateLoad(res, ""))
					}
				case scan.ShrAssign:
					if !g.get(l).IsNil() {
						res := g.builder.CreateAlloca(llvm.Int64Type(), "")
						g.builder.CreateStore(g.builder.CreateAShr(g.get(l), g.get(r), ""), res)
						g.set(l, g.builder.CreateLoad(res, ""))
					}
				case scan.AndNotAssign:
					if !g.get(l).IsNil() {
						res := g.builder.CreateAlloca(llvm.Int64Type(), "")
						nr := g.builder.CreateNeg(g.get(r), "")
						g.builder.CreateStore(g.builder.CreateAnd(g.get(l), nr, ""), res)
						g.set(l, g.builder.CreateLoad(res, ""))
					}
				}
			} else if types.Match(typL, types.String) {
				switch t.Tok.Type {
				case scan.In:
				case scan.Assign:
					sptr := g.builder.CreateAlloca(llvm.PointerType(llvm.Int8Type(), 0), "")
					g.builder.CreateStore(g.get(r), sptr)
					g.set(l, g.builder.CreateLoad(sptr, ""))
				case scan.AddAssign:
					if !g.get(l).IsNil() {
						sptr := g.builder.CreateAlloca(llvm.PointerType(llvm.Int8Type(), 0), "")
						s := g.builder.CreateCall(g.builtin["concat_strings"], []llvm.Value{g.get(l), g.get(r)}, "")
						g.builder.CreateStore(s, sptr)
						g.set(l, g.builder.CreateLoad(sptr, ""))
					}
				}
			}
		}
	case *ast.IfStmt:
		tfn := g.topfn()
		var ip int
		ip, g.ifposq = g.ifposq[0], g.ifposq[1:]
		entry := g.bb[tfn][ip-1]
		T := g.bb[tfn][ip]
		var F llvm.BasicBlock
		T.MoveAfter(entry)
		if t.Else == nil {
			// ip, ip+1
			F = llvm.AddBasicBlock(tfn, "")
			g.bb[tfn] = g.bb[tfn][:ip]
		} else {
			F = g.bb[tfn][ip+1]
			g.bb[tfn] = g.bb[tfn][:ip+1]
		}
		F.MoveAfter(T)
		E := llvm.AddBasicBlock(tfn, "")
		E.MoveAfter(F)
		g.builder.SetInsertPointAtEnd(entry)
		cmp := g.builder.CreateICmp(llvm.IntNE, g.get(t.Cond), llvm.ConstInt(llvm.Int64Type(), 0, false), "")
		g.builder.CreateCondBr(cmp, T, F)
		g.builder.SetInsertPointAtEnd(T)
		g.builder.CreateBr(E)
		g.builder.SetInsertPointAtEnd(F)
		g.builder.CreateBr(E)
		g.builder.SetInsertPointAtEnd(E)
	case *ast.FunDef:
		// switch t.Name.Name.Lit {
		// case "printf", "exit":
		// 	g.popfn()
		// }
	case *ast.File:
		// g.builder.SetInsertPointAtEnd(g.bb[g.topfn()][0])
		ret := g.builder.CreateAlloca(llvm.Int32Type(), "ret")
		g.builder.CreateStore(llvm.ConstInt(llvm.Int32Type(), 0, false), ret)
		retVal := g.builder.CreateLoad(ret, "retVal")
		g.builder.CreateRet(retVal)
	}
	return true
}

func (g *Generator) initLibC() {
	// void exit(int status);
	g.builtin["exit"] = llvm.AddFunction(g.mod, "exit", llvm.FunctionType(
		llvm.VoidType(),
		[]llvm.Type{llvm.Int32Type()},
		false,
	))
	// void *malloc(size_t size);
	g.builtin["malloc"] = llvm.AddFunction(g.mod, "malloc", llvm.FunctionType(
		llvm.PointerType(llvm.Int8Type(), 0),
		[]llvm.Type{llvm.Int64Type()},
		false,
	))
	// void *memcpy(void *dest, const void *src, size_t n);
	g.builtin["memcpy"] = llvm.AddFunction(g.mod, "memcpy", llvm.FunctionType(
		llvm.PointerType(llvm.Int8Type(), 0),
		[]llvm.Type{llvm.PointerType(llvm.Int8Type(), 0), llvm.PointerType(llvm.Int8Type(), 0), llvm.Int64Type()},
		false,
	))
	// int printf(const char *format, ...);
	g.builtin["printf"] = llvm.AddFunction(g.mod, "printf", llvm.FunctionType(
		llvm.Int32Type(),
		[]llvm.Type{llvm.PointerType(llvm.Int8Type(), 0)},
		true,
	))
	// string.h
	// string *alloc_string();
	g.builtin["alloc_string"] = llvm.AddFunction(g.mod, "alloc_string", llvm.FunctionType(
		llvm.PointerType(llvm.Int8Type(), 0),
		[]llvm.Type{},
		false,
	))
	// void init_c_str(string *s, const char *c);
	g.builtin["init_c_str"] = llvm.AddFunction(g.mod, "init_c_str", llvm.FunctionType(
		llvm.VoidType(),
		[]llvm.Type{llvm.PointerType(llvm.Int8Type(), 0), llvm.PointerType(llvm.Int8Type(), 0)},
		false,
	))
	// const char *c_str(string *s);
	g.builtin["c_str"] = llvm.AddFunction(g.mod, "c_str", llvm.FunctionType(
		llvm.PointerType(llvm.Int8Type(), 0),
		[]llvm.Type{llvm.PointerType(llvm.Int8Type(), 0)},
		false,
	))
	// int64_t rune_count(string *s);
	g.builtin["rune_count"] = llvm.AddFunction(g.mod, "rune_count", llvm.FunctionType(
		llvm.Int64Type(),
		[]llvm.Type{llvm.PointerType(llvm.Int8Type(), 0)},
		false,
	))
	// rune index_rune(string *s, int64_t i);
	g.builtin["index_rune"] = llvm.AddFunction(g.mod, "index_rune", llvm.FunctionType(
		llvm.Int32Type(),
		[]llvm.Type{llvm.PointerType(llvm.Int8Type(), 0), llvm.Int64Type()},
		false,
	))
	// void free_string(string *s);
	g.builtin["free_string"] = llvm.AddFunction(g.mod, "free_string", llvm.FunctionType(
		llvm.VoidType(),
		[]llvm.Type{llvm.PointerType(llvm.Int8Type(), 0)},
		false,
	))
	// int32_t assign_string(string *s, int64_t i, string *c);
	g.builtin["assign_string"] = llvm.AddFunction(g.mod, "assign_string", llvm.FunctionType(
		llvm.Int32Type(),
		[]llvm.Type{llvm.PointerType(llvm.Int8Type(), 0), llvm.Int64Type(), llvm.PointerType(llvm.Int8Type(), 0)},
		false,
	))
	// int32_t compare_strings(string *s1, string *s2);
	g.builtin["compare_strings"] = llvm.AddFunction(g.mod, "compare_strings", llvm.FunctionType(
		llvm.Int32Type(),
		[]llvm.Type{llvm.PointerType(llvm.Int8Type(), 0), llvm.PointerType(llvm.Int8Type(), 0)},
		false,
	))
	// string *concat_strings(string *s1, string *s2)
	g.builtin["concat_strings"] = llvm.AddFunction(g.mod, "concat_strings", llvm.FunctionType(
		llvm.PointerType(llvm.Int8Type(), 0),
		[]llvm.Type{llvm.PointerType(llvm.Int8Type(), 0), llvm.PointerType(llvm.Int8Type(), 0)},
		false,
	))
}

func (g *Generator) CreateModule() llvm.Module {
	g.builtin = make(map[string]llvm.Value)
	g.m = make(map[ast.Node]llvm.Value)
	g.bb = make(map[llvm.Value][]llvm.BasicBlock)
	g.builder = llvm.NewBuilder()
	g.mod = llvm.NewModule("module")
	g.initLibC()

	main := llvm.FunctionType(llvm.Int32Type(), []llvm.Type{}, false)
	mainfn := llvm.AddFunction(g.mod, "main", main)
	g.pushfn(mainfn)
	block := llvm.AddBasicBlock(mainfn, "entry")
	g.bb[mainfn] = append(g.bb[mainfn], block)
	g.builder.SetInsertPointAtEnd(block)

	ast.Walk(g.Config.File, g.pre, g.post)

	// verify it's all good
	if ok := llvm.VerifyModule(g.mod, llvm.ReturnStatusAction); ok != nil {
		fmt.Println(ok.Error())
	}

	return g.mod
}
