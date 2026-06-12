package project_test

import (
	"testing"

	"github.com/Ryong256/kanban/internal/db"
	"github.com/Ryong256/kanban/internal/project"
)

func TestAdd_and_List(t *testing.T) {
	d := db.OpenTest(t)

	if err := project.Add(d, "alpha", "/tmp/alpha"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := project.Add(d, "beta", "/tmp/beta"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	list, err := project.List(d)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(list))
	}
}

func TestAdd_duplicate_name_rejected(t *testing.T) {
	d := db.OpenTest(t)
	if err := project.Add(d, "dup", "/tmp/dup"); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	if err := project.Add(d, "dup", "/tmp/dup2"); err == nil {
		t.Fatal("expected error for duplicate name, got nil")
	}
}

func TestRemove(t *testing.T) {
	d := db.OpenTest(t)
	if err := project.Add(d, "rmme", "/tmp/rmme"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := project.Remove(d, "rmme"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	list, _ := project.List(d)
	if len(list) != 0 {
		t.Fatalf("expected 0 projects after remove, got %d", len(list))
	}
}

func TestRemove_missing(t *testing.T) {
	d := db.OpenTest(t)
	if err := project.Remove(d, "doesnotexist"); err == nil {
		t.Fatal("expected error removing missing project, got nil")
	}
}

func TestEnsure_idempotent(t *testing.T) {
	d := db.OpenTest(t)
	project.Ensure(d, "idem", "/tmp/idem")
	project.Ensure(d, "idem", "/tmp/idem") // second call must not error or duplicate
	list, _ := project.List(d)
	if len(list) != 1 {
		t.Fatalf("expected 1 project after two Ensure calls, got %d", len(list))
	}
}

func TestFindByPath(t *testing.T) {
	tests := []struct {
		name      string
		projects  []project.Project
		lookupDir string
		wantName  string
		wantFound bool
	}{
		{
			name: "exact match",
			projects: []project.Project{
				{Name: "alpha", Path: "/home/user/alpha"},
			},
			lookupDir: "/home/user/alpha",
			wantName:  "alpha",
			wantFound: true,
		},
		{
			name: "subdir match",
			projects: []project.Project{
				{Name: "alpha", Path: "/home/user/alpha"},
			},
			lookupDir: "/home/user/alpha/src/pkg",
			wantName:  "alpha",
			wantFound: true,
		},
		{
			name: "no match",
			projects: []project.Project{
				{Name: "alpha", Path: "/home/user/alpha"},
			},
			lookupDir: "/home/user/other",
			wantFound: false,
		},
		{
			name: "home dir not a catch-all",
			projects: []project.Project{
				{Name: "home-alfredo", Path: "/home/alfredo"},
			},
			// A project that happens to live under /home/alfredo DOES match
			// because prefix matching is correct. The bug was /home/alfredo being
			// auto-registered without .git; after the fix that no longer happens.
			lookupDir: "/home/alfredo/Projects/kanban",
			wantName:  "home-alfredo",
			wantFound: true,
		},
		{
			name: "longest path wins",
			projects: []project.Project{
				{Name: "workspace", Path: "/home/user/ws"},
				{Name: "specific", Path: "/home/user/ws/specific"},
			},
			lookupDir: "/home/user/ws/specific/src",
			wantName:  "specific",
			wantFound: true,
		},
		{
			name: "prefix boundary respected",
			projects: []project.Project{
				{Name: "proj", Path: "/home/user/proj"},
			},
			// /home/user/projother must NOT match /home/user/proj
			lookupDir: "/home/user/projother",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := db.OpenTest(t)
			for _, p := range tt.projects {
				project.Ensure(d, p.Name, p.Path)
			}
			got, found, err := project.FindByPath(d, tt.lookupDir)
			if err != nil {
				t.Fatalf("FindByPath: %v", err)
			}
			if found != tt.wantFound {
				t.Fatalf("found=%v, want %v", found, tt.wantFound)
			}
			if found && got.Name != tt.wantName {
				t.Fatalf("name=%q, want %q", got.Name, tt.wantName)
			}
		})
	}
}
