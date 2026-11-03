package main

import (
	"encoding/json"
	"fmt"
	"strings"

	loneproject "lone/pkg/lone-analyzer-project"

	"github.com/spf13/cobra"
)

var (
	debugJsonFlag bool
)

var debugCmd = &cobra.Command{
	Use:   "debug <command|scope> [symbol]",
	Short: "Inspect scopes, symbols, memory layout, traits, and storage locations",
	Long: `The debug command provides deep inspection of Lone symbols and scopes.

Subcommands:
  dump <scope>            Dumps an overview of all symbols in the scope (structs, traits, functions, globals, imports).
  whatis <scope> <symbol> Displays symbol kind, defined location, memory layout, traits, or variable storage.

Shorthand:
  lone debug <scope>          Alias for 'lone debug dump <scope>'
  lone debug <scope> <symbol> Alias for 'lone debug whatis <scope> <symbol>'

Scopes can be a Lone source file (e.g. 'main.lone') or a module namespace (e.g. ':std:io').
Symbols can be unscoped (e.g. 'Stdout') or scoped with ':' or '.' (e.g. 'main:guess', 'Stdout:write').`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			// lone debug <scope> -> dump <scope>
			return runDebugDump(args[0])
		}
		// lone debug <scope> <symbol> -> whatis <scope> <symbol>
		return runDebugWhatis(args[0], args[1])
	},
}

var debugDumpCmd = &cobra.Command{
	Use:   "dump <scope>",
	Short: "Dump an overview of all symbols in a file or namespace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDebugDump(args[0])
	},
}

var debugWhatisCmd = &cobra.Command{
	Use:     "whatis <scope> <symbol>",
	Aliases: []string{"inspect", "info"},
	Short:   "Inspect a symbol's type, memory layout, traits, or storage location",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDebugWhatis(args[0], args[1])
	},
}

func init() {
	debugCmd.PersistentFlags().BoolVar(&debugJsonFlag, "json", false, "Output results in JSON format")
	debugCmd.AddCommand(debugDumpCmd)
	debugCmd.AddCommand(debugWhatisCmd)
}

func runDebugDump(scope string) error {
	dump, err := loneproject.DumpScope(scope)
	if err != nil {
		return err
	}

	if debugJsonFlag {
		data, err := json.MarshalIndent(dump, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Scope:    %s\n", dump.Scope)
	fmt.Printf("File:     %s\n\n", dump.FilePath)

	// Structs
	fmt.Printf("Structs (%d):\n", len(dump.Structs))
	if len(dump.Structs) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, s := range dump.Structs {
			vis := "internal"
			if s.IsPublic {
				vis = "pub"
			}
			fmt.Printf("  • %-16s [size: %2dB, align: %2dB, fields: %d, methods: %d, traits: %d, %s]\n",
				s.Name, s.Size, s.Align, s.FieldCount, s.MethodCount, s.TraitCount, vis)
		}
	}
	fmt.Println()

	// Traits
	fmt.Printf("Traits (%d):\n", len(dump.Traits))
	if len(dump.Traits) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, tr := range dump.Traits {
			vis := "internal"
			if tr.IsPublic {
				vis = "pub"
			}
			impls := "none"
			if len(tr.Implementors) > 0 {
				impls = strings.Join(tr.Implementors, ", ")
			}
			fmt.Printf("  • %-16s [methods: %d, implementors: [%s], %s]\n",
				tr.Name, tr.MethodCount, impls, vis)
		}
	}
	fmt.Println()

	// Functions
	fmt.Printf("Functions (%d):\n", len(dump.Functions))
	if len(dump.Functions) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, fn := range dump.Functions {
			pubPrefix := ""
			if fn.IsPublic {
				pubPrefix = "pub "
			}
			fmt.Printf("  • %s%s (locals: %d)\n", pubPrefix, fn.Signature, fn.LocalCount)
		}
	}
	fmt.Println()

	// Globals
	fmt.Printf("Globals (%d):\n", len(dump.Globals))
	if len(dump.Globals) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, g := range dump.Globals {
			addrNote := ""
			if g.AddressTaken {
				addrNote = ", address-taken"
			}
			fmt.Printf("  • %s: %s [%s%s]\n", g.Name, g.Type, g.Storage, addrNote)
		}
	}
	fmt.Println()

	// Imports
	fmt.Printf("Imports (%d):\n", len(dump.Imports))
	if len(dump.Imports) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, imp := range dump.Imports {
			if imp.Alias != "" {
				fmt.Printf("  • %s as %s\n", imp.Source, imp.Alias)
			} else if len(imp.Symbols) > 0 {
				fmt.Printf("  • from %s (%s)\n", imp.Source, strings.Join(imp.Symbols, ", "))
			} else {
				fmt.Printf("  • %s\n", imp.Source)
			}
		}
	}

	return nil
}

func runDebugWhatis(scope, symbol string) error {
	info, err := loneproject.InspectSymbol(scope, symbol)
	if err != nil {
		return err
	}

	if debugJsonFlag {
		data, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Symbol:      %s\n", info.Name)
	if info.ScopedName != "" && info.ScopedName != info.Name {
		fmt.Printf("Scoped:      %s\n", info.ScopedName)
	}

	vis := "internal"
	if info.IsPublic {
		vis = "public"
	}
	fmt.Printf("Kind:        %s (%s)\n", info.Kind, vis)
	if info.DefinedPos != "" {
		fmt.Printf("Defined:     %s\n", info.DefinedPos)
	}
	fmt.Printf("Scope:       %s\n", info.Scope)

	switch info.Kind {
	case loneproject.DebugKindStruct:
		fmt.Printf("Memory:      Size = %d bytes, Align = %d bytes\n\n", info.Size, info.Align)

		fmt.Printf("Fields (%d):\n", len(info.Fields))
		if len(info.Fields) == 0 {
			fmt.Println("  (none)")
		} else {
			for _, f := range info.Fields {
				fmt.Printf("  +%-3d %-16s %-16s (size %dB)\n", f.Offset, f.Name+":", f.Type, f.Size)
			}
		}
		fmt.Println()

		if len(info.Constructors) > 0 {
			fmt.Printf("Constructors (%d):\n", len(info.Constructors))
			for _, c := range info.Constructors {
				fmt.Printf("  • %s\n", c)
			}
			fmt.Println()
		}

		if len(info.Methods) > 0 {
			fmt.Printf("Inherent Methods (%d):\n", len(info.Methods))
			for _, m := range info.Methods {
				staticNote := ""
				if m.IsStatic {
					staticNote = " (static)"
				}
				fmt.Printf("  • %s%s\n", m.Signature, staticNote)
			}
			fmt.Println()
		}

		fmt.Printf("Implemented Traits (%d):\n", len(info.ImplementedTraits))
		if len(info.ImplementedTraits) == 0 {
			fmt.Println("  (none)")
		} else {
			for _, t := range info.ImplementedTraits {
				fmt.Printf("  • %s:\n", t.TraitName)
				for _, m := range t.Methods {
					fmt.Printf("    - %s\n", m.Signature)
				}
			}
		}

	case loneproject.DebugKindTrait:
		fmt.Println()
		fmt.Printf("Method Contracts (%d):\n", len(info.TraitMethods))
		if len(info.TraitMethods) == 0 {
			fmt.Println("  (none)")
		} else {
			for _, m := range info.TraitMethods {
				fmt.Printf("  • %s\n", m.Signature)
			}
		}
		fmt.Println()

		fmt.Printf("Concrete Implementors (%d):\n", len(info.Implementors))
		if len(info.Implementors) == 0 {
			fmt.Println("  (none)")
		} else {
			for _, imp := range info.Implementors {
				fmt.Printf("  • %s\n", imp)
			}
		}

	case loneproject.DebugKindVar:
		if info.EnclosingFn != "" {
			fmt.Printf("Enclosing:   %s\n", info.EnclosingFn)
		}
		fmt.Printf("Type:        %s\n", info.ResolvedType)
		initStr := "no"
		if info.IsInitialized {
			initStr = "yes"
		}
		fmt.Printf("Initialized: %s\n", initStr)
		fmt.Printf("Storage:     %s\n", info.Storage)
		addrStr := "no (&raw was not applied)"
		if info.AddressTaken {
			addrStr = "yes (&raw was applied)"
		}
		fmt.Printf("AddressTaken:%s\n", " "+addrStr)

	case loneproject.DebugKindFn:
		fmt.Printf("Signature:   %s\n", info.Signature)
		fmt.Printf("Return:      %s\n\n", info.ReturnType)

		fmt.Printf("Parameters (%d):\n", len(info.Params))
		if len(info.Params) == 0 {
			fmt.Println("  (none)")
		} else {
			for _, p := range info.Params {
				addr := "no"
				if p.AddressTaken {
					addr = "yes"
				}
				fmt.Printf("  • %s: %s [%s, address_taken: %s]\n", p.Name, p.Type, p.Storage, addr)
			}
		}
		fmt.Println()

		fmt.Printf("Local Variables (%d):\n", len(info.LocalVars))
		if len(info.LocalVars) == 0 {
			fmt.Println("  (none)")
		} else {
			for _, v := range info.LocalVars {
				addr := "no"
				if v.AddressTaken {
					addr = "yes"
				}
				fmt.Printf("  • %s: %s [%s, address_taken: %s]\n", v.Name, v.Type, v.Storage, addr)
			}
		}

	case loneproject.DebugKindField:
		fmt.Printf("Struct:      %s\n", info.ParentStruct)
		fmt.Printf("Type:        %s\n", info.ResolvedType)
		fmt.Printf("Offset:      +%d bytes\n", info.FieldOffset)
		fmt.Printf("Size:        %d bytes\n", info.FieldSize)

	case loneproject.DebugKindMethod:
		fmt.Printf("Signature:   %s\n", info.Signature)
		if info.TraitSource != "" {
			fmt.Printf("Trait:       %s\n", info.TraitSource)
		}
	}

	return nil
}
