package cli

import (
	"bufio"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v3"

	rutils "go.viam.com/rdk/utils"
)

// The skill is a copy of app's skills/catalog/using-viam, which the Viam MCP server serves.
//
//go:embed skills/using-viam
var embeddedSkills embed.FS

const (
	viamSkillName        = "using-viam"
	embeddedSkillRoot    = "skills/" + viamSkillName
	skillsFlagDir        = "dir"
	skillsStateFileName  = "cli_skills.json"
	skillsStagingPattern = ".using-viam-*"
)

// agentDirs are the per-user folders of agents that load Agent Skills. A skill is installed under
// <home>/<dir>/skills only when <home>/<dir> already exists, so no folder is made for an absent agent.
var agentDirs = []string{".claude", ".agents"}

var (
	skillsHomeDir   = os.UserHomeDir
	skillsStatePath = func() string { return filepath.Join(rutils.ViamDotDir, skillsStateFileName) }
	// Both ends must be a terminal: with stdout redirected, the question would land in the output.
	skillPromptInteractive = func() bool { return isInteractive() && isOutputTTY() }
)

type skillsState struct {
	Prompted  bool             `json:"prompted"`
	Installed []installedSkill `json:"installed,omitempty"`
}

type installedSkill struct {
	Dir    string `json:"dir"`
	Digest string `json:"digest"`
}

func readSkillsState() (*skillsState, error) {
	b, err := os.ReadFile(skillsStatePath())
	if os.IsNotExist(err) {
		return &skillsState{}, nil
	}
	if err != nil {
		return nil, err
	}
	var s skillsState
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, errors.Wrapf(err, "failed to parse %s", skillsStatePath())
	}
	return &s, nil
}

func (s *skillsState) write() error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(skillsStatePath()), 0o700); err != nil {
		return err
	}
	//nolint:gosec
	return os.WriteFile(skillsStatePath(), b, 0o640)
}

func (s *skillsState) recordInstall(dir, digest string) {
	for i := range s.Installed {
		if s.Installed[i].Dir == dir {
			s.Installed[i].Digest = digest
			return
		}
	}
	s.Installed = append(s.Installed, installedSkill{Dir: dir, Digest: digest})
}

// embeddedSkillDigest identifies this binary's copy of the skill, so an installed copy from an
// older CLI can be told apart and refreshed.
func embeddedSkillDigest() (string, error) {
	h := sha256.New()
	err := fs.WalkDir(embeddedSkills, embeddedSkillRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := embeddedSkills.ReadFile(p)
		if err != nil {
			return err
		}
		h.Write([]byte(fmt.Sprintf("%s\x00%d\x00", p, len(b))))
		h.Write(b)
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// detectSkillsDirs returns the skills folder of each agent found in the home directory.
func detectSkillsDirs() ([]string, error) {
	home, err := skillsHomeDir()
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, agentDir := range agentDirs {
		if info, err := os.Stat(filepath.Join(home, agentDir)); err == nil && info.IsDir() {
			dirs = append(dirs, filepath.Join(home, agentDir, "skills"))
		}
	}
	return dirs, nil
}

// installSkill writes the skill to <skillsDir>/using-viam, replacing any earlier copy whole so files
// dropped from the skill do not linger. It returns the skill's folder.
func installSkill(skillsDir string) (string, error) {
	dest := filepath.Join(skillsDir, viamSkillName)
	if err := os.MkdirAll(skillsDir, 0o750); err != nil {
		return "", err
	}
	staging, err := os.MkdirTemp(skillsDir, skillsStagingPattern)
	if err != nil {
		return "", err
	}
	defer func() {
		//nolint:errcheck
		os.RemoveAll(staging)
	}()

	err = fs.WalkDir(embeddedSkills, embeddedSkillRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(p, embeddedSkillRoot), "/")
		target := filepath.Join(staging, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		b, err := embeddedSkills.ReadFile(p)
		if err != nil {
			return err
		}
		//nolint:gosec
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		return "", err
	}
	//nolint:gosec
	if err := os.Chmod(staging, 0o755); err != nil {
		return "", err
	}

	if _, err := os.Stat(dest); err == nil {
		old, err := os.MkdirTemp(skillsDir, skillsStagingPattern)
		if err != nil {
			return "", err
		}
		oldDest := filepath.Join(old, viamSkillName)
		defer func() {
			//nolint:errcheck
			os.RemoveAll(old)
		}()
		if err := os.Rename(dest, oldDest); err != nil {
			return "", err
		}
		if err := os.Rename(staging, dest); err != nil {
			//nolint:errcheck
			os.Rename(oldDest, dest)
			return "", err
		}
		return dest, nil
	}
	if err := os.Rename(staging, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// installSkillTo installs into each skills folder and records the installs, so later CLI versions
// refresh them.
func installSkillTo(cmd *cli.Command, skillsDirs []string) error {
	digest, err := embeddedSkillDigest()
	if err != nil {
		return err
	}
	state, err := readSkillsState()
	if err != nil {
		return err
	}
	for _, dir := range skillsDirs {
		dest, err := installSkill(dir)
		if err != nil {
			return errors.Wrapf(err, "failed to install the Viam skill to %s", dir)
		}
		state.recordInstall(dest, digest)
		printf(cmd.Root().Writer, "Installed the Viam skill to %s", dest)
	}
	state.Prompted = true
	return state.write()
}

type skillsInstallArgs struct {
	Dir string
}

// SkillsInstallAction is the corresponding Action for 'skills install'.
func SkillsInstallAction(ctx context.Context, cmd *cli.Command, args skillsInstallArgs) error {
	if args.Dir != "" {
		dir, err := filepath.Abs(args.Dir)
		if err != nil {
			return err
		}
		return installSkillTo(cmd, []string{dir})
	}
	dirs, err := detectSkillsDirs()
	if err != nil {
		return err
	}
	if len(dirs) == 0 {
		return errors.Errorf("found no agent folder (%s) in your home directory; pass --%s=<skills folder> to install "+
			"the Viam skill somewhere else", strings.Join(agentDirs, ", "), skillsFlagDir)
	}
	return installSkillTo(cmd, dirs)
}

// offerSkillInstall asks once, in an interactive terminal, whether to install the skill for the
// agents found. Its failures are warnings so they never fail the command it follows.
func offerSkillInstall(cmd *cli.Command) {
	globalArgs, err := getGlobalArgs(cmd)
	if err != nil || globalArgs.Quiet || !skillPromptInteractive() {
		return
	}
	state, err := readSkillsState()
	if err != nil || state.Prompted {
		return
	}
	dirs, err := detectSkillsDirs()
	if err != nil || len(dirs) == 0 {
		return
	}

	printf(cmd.Root().Writer, "\nInstall the Viam skill for AI coding agents to %s? It teaches agents how to use Viam "+
		"and the viam CLI. [y/N]:", strings.Join(dirs, " and "))
	answer, err := bufio.NewReader(cmd.Root().Reader).ReadString('\n')
	if err != nil {
		return
	}
	if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
		state.Prompted = true
		if err := state.write(); err != nil {
			warningf(cmd.Root().ErrWriter, "failed to save your answer: %v", err)
		}
		printf(cmd.Root().Writer, "Skipped. Run 'viam skills install' to install it later.")
		return
	}
	if err := installSkillTo(cmd, dirs); err != nil {
		warningf(cmd.Root().ErrWriter, "%v", err)
	}
}

// OfferSkillInstall asks once whether to install the skill. main calls it after a command succeeds,
// so the question follows the command's own output.
func OfferSkillInstall(app *cli.Command) {
	offerSkillInstall(app)
}

// refreshInstalledSkills rewrites installed copies made by an older CLI. A copy whose folder was
// deleted is forgotten rather than restored.
func refreshInstalledSkills(cmd *cli.Command) {
	state, err := readSkillsState()
	if err != nil || len(state.Installed) == 0 {
		return
	}
	digest, err := embeddedSkillDigest()
	if err != nil {
		return
	}
	changed := false
	kept := state.Installed[:0]
	for _, inst := range state.Installed {
		if _, err := os.Stat(inst.Dir); err != nil {
			changed = true
			continue
		}
		if inst.Digest != digest {
			if _, err := installSkill(filepath.Dir(inst.Dir)); err != nil {
				debugf(cmd.Root().ErrWriter, isDebug(cmd), "failed to refresh the Viam skill in %s: %v", inst.Dir, err)
				kept = append(kept, inst)
				continue
			}
			inst.Digest = digest
			changed = true
		}
		kept = append(kept, inst)
	}
	if !changed {
		return
	}
	state.Installed = kept
	if err := state.write(); err != nil {
		debugf(cmd.Root().ErrWriter, isDebug(cmd), "failed to save the Viam skill state: %v", err)
	}
}

func isDebug(cmd *cli.Command) bool {
	globalArgs, err := getGlobalArgs(cmd)
	return err == nil && globalArgs.Debug
}

// refreshSkillsBefore is the app's Before hook.
func refreshSkillsBefore(ctx context.Context, cmd *cli.Command) (context.Context, error) {
	refreshInstalledSkills(cmd)
	return ctx, nil
}
