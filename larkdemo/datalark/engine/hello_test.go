package datalarkengine

import (
	"fmt"

	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/schema"
	"go.starlark.net/starlark"
)

/*
Remarks on starlark modules:

- Things starting with "_" are not exported.  (The starlark 'load' will refuse to create references to them.)
- I still wonder if I can wrap 'load'.
	- I'm still not thrilled with the sheer weirdness of syntax in `load("module.star", "x", y2="y", "z")    # assigns x, y2, and z`.
	- I wonder if that will make more things seen as dynamically referenced, and allow fewer nice relatively-ahead-of-time checks.  I think so.
		- Not sure if care.
			- We *need* dotted access to keep datalark's symbol set remotely reasonable.  That means almost everything will be seen as dynamic already anyway.
	- Does 'load' also cause things to be globally available in the module while *not* adding them to exports?  That's also relevant.
		- If I go with the explicit 'export' var idea, we're on solid ground again, but we might indeed need both for a 'load' wrapper that makes it act more like regular assignment to be viable.
- There is, apparently, no "*" for 'load'.
	- I have a very love/hate relationship with this.
	- On the one hand: love, because explicit is good.
	- On the other hand: hate, because there's also no "with" or "do" syntax.
	- And on the gripping hand: hate, because it defacto causes the community of starlark embedders to just yeet things in the global namespace anyway... just doing so using more magic than the user themselves can have.  This does not result in better legibility, imo, since once side just doesn't have to (and regularly, doesn't) play by the rules.

- What modules communicate is indeed a `starlark.StringDict` -- the "globals" returned by `starlark.ExecFile`.
	- See https://github.com/google/starlark-go/blob/7a1108eaa0124ea025e567d9feb4e41aab4bb024/starlark/example_test.go#L89 for example.
	- So, okay, no: there is apparently no need for us to actually eval any kind of module; we can certainly do a logically equivalent thing by ourselves.
- It perturbs me a great deal that `starlark.StringDict` doesn't bother to implement `starlark.Value`, and thus nesting is weird.
	- It just... didn't have to be?
	- Whatever.

---

Far future:

I could see wanting to build a module which lets someone process schemas, _starting **from** starlark_,
and receives new TypedPrototype values, and can go to town with those.

(This would require quite a bit of work in various places to make sure that's actually useful;
for example, making sure you can slam together schemas and something like basicnode that has no golang host type specificity.
While some of that is not present and fully wired today, it's all things that would be well-defined and we'd like to do someday;
so let's design with it in mind.)

Couple things about this:

- Could we replicate the whole node|type|prototype relationship?  Yes.  Should we?  Unclear.
	- I can't actually think of a great reason; why would you ever have a type description floating around in starlark that you can't initialize?  And when would you really need to distinguish different prototypes without also being sort of indifferent to duplicating the type info?
		- The same could be said of golang, but there, I think the package relationships came out meaningfully clearer by separating these things; we needed more interfaces and more gaps to actually be able to implement unrelated node prototypes in unrelated packages.  The constraints don't necessarily translate to starlark the same way.
		- Presumably, in starlark, we'd always give the type info a default constructor behavior which does something that wraps types around basicnode.  There are no other useful options.  (If someone hands you *in* things that are wrappers using bindnode or codegen'd nodes, that's also fine... but you can't meaningfully _start_ that road from pure starlark.)
- So, ultimately, is there any point to having a 'Prototype' value, and attaching a 'make' function to it?
	- If we can make something callable, but also have attributes, in starlark: then definitely, go for that.  Why not?
		- I don't think we can do this, to my disappointment.  The `starlark.Builtin` type is... not powerful enough.
			- ... at least, not by itself.  There is a `starlark.Callable` interface.  Maybe we can make a custom type that does what we want, here.
	- I think if we had a module that could reason about schemas and compile them itself, it would return Prototype values, yeah.
	- I don't know if the above is an argument that wrapped constructors handed in from outside need to be attached to a Prototype.
		- I just... can't really see a reason for someone to want to inspect this from starlark.
			- Even if IPLD starts down the road of attaching more feature-detection points to NodePrototype... our starlark wrapper functions can do those, and make them idiomatic to starlark.

...

okay, let's bring these questions back down to the ground.

- Are there types in golang for prototypes?  Or are we just wrapping constructor functions into `starlark.Builtin` and that's it?
- Do I want one type for all prototypes?  Or is it worth having a separate one per typekind?
- What does this mean for the names we attach to builtins?

Answers:

- Let's try the `Callable` heavy-firepower approach.  I like it.  It gives us the most power and the most future extensibility.
	- (Bonus: more control about what the debug string methods say than just using a builtin bare, which is very poor on that front.)
- Let's give up and make one megatype.  There seems to be a lot of boilerplate if making a separate one per typekind.
	- (At some point, I thought having more golang types would result in better debuggability, but ... no, actually, it doesn't really seem to help in practice.)
	- N.B. we can do one megatype for the prototypes.  We still need individual types for the read wrappers, because starlark is doing feature detection on what methods they expose, rather than having a method where we can *say* what kind we are.
- Naming is hard, let's just start rolling and see where we can get.  Let's be liberal with dots and prefixes for now and we can iterate and trim later.
	- The name for anything that's a builtin should be long-ish, and the name we bind it to can be much shorter.
	- The name on builtins will matter a lot less given our heavy-firepower approach to the first question.
	- (If we allow user-generated schemas, at some point we'll have to wrangle how we differentiate types of the same name but from different universes.  But... we've actually not dealt with that anywhere in IPLD yet, so let's not get hung up here prematurely either.)

*/

func eval(src string, tsname string, npts []schema.TypedPrototype) {
	thread := &starlark.Thread{
		Name: "thethreadname",
		Print: func(thread *starlark.Thread, msg string) {
			//caller := thread.CallFrame(1)
			//fmt.Printf("%s: %s: %s\n", caller.Pos, caller.Name, msg)
			fmt.Printf("%s\n", msg)
		},
	}
	_, err := starlark.ExecFile(thread, "thefilename.star", src,
		starlark.StringDict{
			"String": &Prototype{basicnode.Prototype.String},
			"Map":    &Prototype{basicnode.Prototype.Map},
			tsname:   constructorMap(npts),
		},
	)
	if err != nil {
		panic(err)
	}
}

// You can't just range over a schema.TypeSystem and do everything -- because we need to know what the implementation and memory layout is going to be.
// That means we need prototypes.  And those prototypes can't just be pulled out of thin air.
func constructorMap(npts []schema.TypedPrototype) *Object {
	d := NewObject(len(npts))
	for _, npt := range npts {
		d.SetKey(starlark.String(npt.Type().Name()), &Prototype{npt})
	}
	d.Freeze()
	return d
}

func Example_hello() {
	eval(`
print(String)
print(String("yo"))
x = {"bz": "zoo"}
print(Map(hey="hai", zonk="wot", **x))
#print(Map({String("fun"): "heeey"}))
`, "", nil)

	// Output:
	// <built-in function datalark.Prototype>
	// string{"yo"}
	// map{
	// 	string{"hey"}: string{"hai"}
	// 	string{"zonk"}: string{"wot"}
	// 	string{"bz"}: string{"zoo"}
	// }
}

func Example_structs() {
	ts := schema.MustTypeSystem(
		schema.SpawnString("String"),
		schema.SpawnStruct("FooBar", []schema.StructField{
			schema.SpawnStructField("foo", "String", false, false),
			schema.SpawnStructField("bar", "String", false, false),
		}, nil),
	)
	type FooBar struct{ Foo, Bar string }

	eval(`
#print(dir(ts))
print(ts.FooBar)
print(ts.FooBar(foo="hai", bar="wot"))
`, "ts", []schema.TypedPrototype{
		bindnode.Prototype((*FooBar)(nil), ts.TypeByName("FooBar")),
	})

	// Output:
	// <built-in function datalark.Prototype<FooBar>>
	// struct<FooBar>{
	// 	foo: string<String>{"hai"}
	// 	bar: string<String>{"wot"}
	// }
}
