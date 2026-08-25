package template

import "testing"

func TestSetDirAnswersFromTheNewFolder(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	c, err := NewCatalog(first)
	if err != nil {
		t.Fatal(err)
	}

	original, _ := c.Get("minecraft-java")
	edited := original
	edited.DefaultMemory = "8g"
	if err := c.Save(edited); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := c.SetDir(second); err != nil {
		t.Fatalf("SetDir: %v", err)
	}
	if got, _ := c.Get("minecraft-java"); got.DefaultMemory != original.DefaultMemory {
		t.Errorf("the edit of the old folder followed to the new one: %q", got.DefaultMemory)
	}

	// what was written on the first folder is still there, moving the panel does not move files
	if err := c.SetDir(first); err != nil {
		t.Fatalf("SetDir back: %v", err)
	}
	if got, _ := c.Get("minecraft-java"); got.DefaultMemory != "8g" {
		t.Errorf("going back lost the template: %q", got.DefaultMemory)
	}
}
