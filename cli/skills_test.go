package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
	"go.viam.com/test"
)

// useFakeSkillsHome points skill detection and the state file at temp folders and returns the home.
func useFakeSkillsHome(t *testing.T, agentFolders ...string) string {
	t.Helper()
	home := t.TempDir()
	statePath := filepath.Join(t.TempDir(), skillsStateFileName)
	for _, f := range agentFolders {
		test.That(t, os.MkdirAll(filepath.Join(home, f), 0o750), test.ShouldBeNil)
	}
	oldHome, oldState, oldInteractive := skillsHomeDir, skillsStatePath, skillPromptInteractive
	skillsHomeDir = func() (string, error) { return home, nil }
	skillsStatePath = func() string { return statePath }
	t.Cleanup(func() {
		skillsHomeDir, skillsStatePath, skillPromptInteractive = oldHome, oldState, oldInteractive
	})
	return home
}

func newSkillsTestCommand(input string) (*cli.Command, *bytes.Buffer) {
	out := &bytes.Buffer{}
	cmd := &cli.Command{Writer: out, ErrWriter: out, Reader: strings.NewReader(input)}
	return cmd, out
}

func TestEmbeddedSkillIsValid(t *testing.T) {
	b, err := embeddedSkills.ReadFile(embeddedSkillRoot + "/SKILL.md")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(b), test.ShouldStartWith, "---\nname: "+viamSkillName+"\n")
	test.That(t, string(b), test.ShouldNotContainSubstring, "TODO")
}

func TestDetectSkillsDirs(t *testing.T) {
	home := useFakeSkillsHome(t, ".claude")
	dirs, err := detectSkillsDirs()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dirs, test.ShouldResemble, []string{filepath.Join(home, ".claude", "skills")})

	useFakeSkillsHome(t)
	dirs, err = detectSkillsDirs()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dirs, test.ShouldBeEmpty)
}

func TestInstallSkillReplacesEarlierCopy(t *testing.T) {
	skillsDir := t.TempDir()
	stale := filepath.Join(skillsDir, viamSkillName, "references", "removed.md")
	test.That(t, os.MkdirAll(filepath.Dir(stale), 0o750), test.ShouldBeNil)
	test.That(t, os.WriteFile(stale, []byte("old"), 0o600), test.ShouldBeNil)

	dest, err := installSkill(skillsDir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dest, test.ShouldEqual, filepath.Join(skillsDir, viamSkillName))

	want, err := embeddedSkills.ReadFile(embeddedSkillRoot + "/SKILL.md")
	test.That(t, err, test.ShouldBeNil)
	got, err := os.ReadFile(filepath.Join(dest, "SKILL.md"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldResemble, want)
	_, err = os.Stat(filepath.Join(dest, "references", "modules.md"))
	test.That(t, err, test.ShouldBeNil)
	_, err = os.Stat(stale)
	test.That(t, os.IsNotExist(err), test.ShouldBeTrue)

	entries, err := os.ReadDir(skillsDir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(entries), test.ShouldEqual, 1)
}

func TestSkillsInstallAction(t *testing.T) {
	t.Run("installs into each detected agent folder", func(t *testing.T) {
		home := useFakeSkillsHome(t, ".claude", ".agents")
		cmd, out := newSkillsTestCommand("")
		test.That(t, SkillsInstallAction(context.Background(), cmd, skillsInstallArgs{}), test.ShouldBeNil)
		for _, agent := range []string{".claude", ".agents"} {
			dest := filepath.Join(home, agent, "skills", viamSkillName)
			_, err := os.Stat(filepath.Join(dest, "SKILL.md"))
			test.That(t, err, test.ShouldBeNil)
			test.That(t, out.String(), test.ShouldContainSubstring, "Installed the Viam skill to "+dest)
		}
		state, err := readSkillsState()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, state.Prompted, test.ShouldBeTrue)
		test.That(t, len(state.Installed), test.ShouldEqual, 2)
	})

	t.Run("installs into --dir", func(t *testing.T) {
		useFakeSkillsHome(t)
		dir := t.TempDir()
		cmd, _ := newSkillsTestCommand("")
		test.That(t, SkillsInstallAction(context.Background(), cmd, skillsInstallArgs{Dir: dir}), test.ShouldBeNil)
		_, err := os.Stat(filepath.Join(dir, viamSkillName, "SKILL.md"))
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("fails with no agent folder and no --dir", func(t *testing.T) {
		useFakeSkillsHome(t)
		cmd, _ := newSkillsTestCommand("")
		err := SkillsInstallAction(context.Background(), cmd, skillsInstallArgs{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "--dir")
	})
}

func TestOfferSkillInstall(t *testing.T) {
	t.Run("yes installs and is not asked again", func(t *testing.T) {
		home := useFakeSkillsHome(t, ".claude")
		skillPromptInteractive = func() bool { return true }
		cmd, out := newSkillsTestCommand("y\n")
		offerSkillInstall(cmd)
		test.That(t, out.String(), test.ShouldContainSubstring, "Install the Viam skill")
		_, err := os.Stat(filepath.Join(home, ".claude", "skills", viamSkillName, "SKILL.md"))
		test.That(t, err, test.ShouldBeNil)

		cmd, out = newSkillsTestCommand("y\n")
		offerSkillInstall(cmd)
		test.That(t, out.String(), test.ShouldBeEmpty)
	})

	t.Run("no installs nothing and is not asked again", func(t *testing.T) {
		home := useFakeSkillsHome(t, ".claude")
		skillPromptInteractive = func() bool { return true }
		cmd, out := newSkillsTestCommand("n\n")
		offerSkillInstall(cmd)
		test.That(t, out.String(), test.ShouldContainSubstring, "viam skills install")
		_, err := os.Stat(filepath.Join(home, ".claude", "skills"))
		test.That(t, os.IsNotExist(err), test.ShouldBeTrue)

		cmd, out = newSkillsTestCommand("y\n")
		offerSkillInstall(cmd)
		test.That(t, out.String(), test.ShouldBeEmpty)
	})

	t.Run("does not ask without a terminal", func(t *testing.T) {
		useFakeSkillsHome(t, ".claude")
		skillPromptInteractive = func() bool { return false }
		cmd, out := newSkillsTestCommand("y\n")
		offerSkillInstall(cmd)
		test.That(t, out.String(), test.ShouldBeEmpty)
		state, err := readSkillsState()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, state.Prompted, test.ShouldBeFalse)
	})

	t.Run("does not ask with no agent folder", func(t *testing.T) {
		useFakeSkillsHome(t)
		skillPromptInteractive = func() bool { return true }
		cmd, out := newSkillsTestCommand("y\n")
		offerSkillInstall(cmd)
		test.That(t, out.String(), test.ShouldBeEmpty)
	})
}

func TestRefreshInstalledSkills(t *testing.T) {
	home := useFakeSkillsHome(t, ".claude", ".agents")
	cmd, _ := newSkillsTestCommand("")
	test.That(t, SkillsInstallAction(context.Background(), cmd, skillsInstallArgs{}), test.ShouldBeNil)

	claudeSkill := filepath.Join(home, ".claude", "skills", viamSkillName)
	agentsSkill := filepath.Join(home, ".agents", "skills", viamSkillName)
	test.That(t, os.WriteFile(filepath.Join(claudeSkill, "SKILL.md"), []byte("old"), 0o600), test.ShouldBeNil)
	test.That(t, os.RemoveAll(agentsSkill), test.ShouldBeNil)
	state, err := readSkillsState()
	test.That(t, err, test.ShouldBeNil)
	for i := range state.Installed {
		state.Installed[i].Digest = "from-an-older-cli"
	}
	test.That(t, state.write(), test.ShouldBeNil)

	refreshInstalledSkills(cmd)

	want, err := embeddedSkills.ReadFile(embeddedSkillRoot + "/SKILL.md")
	test.That(t, err, test.ShouldBeNil)
	got, err := os.ReadFile(filepath.Join(claudeSkill, "SKILL.md"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldResemble, want)

	_, err = os.Stat(agentsSkill)
	test.That(t, os.IsNotExist(err), test.ShouldBeTrue)
	state, err = readSkillsState()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, state.Installed, test.ShouldResemble, []installedSkill{{Dir: claudeSkill, Digest: state.Installed[0].Digest}})
	test.That(t, state.Installed[0].Digest, test.ShouldNotEqual, "from-an-older-cli")
}
