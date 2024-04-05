package db_lib

import (
	"crypto/md5"
	"fmt"
	"github.com/ansible-semaphore/semaphore/db"
	"github.com/ansible-semaphore/semaphore/lib"
	"io"
	"os"
	"path"
)

func getMD5Hash(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func hasRequirementsChanges(requirementsFilePath string, requirementsHashFilePath string) bool {
	oldFileMD5HashBytes, err := os.ReadFile(requirementsHashFilePath)
	if err != nil {
		return true
	}

	newFileMD5Hash, err := getMD5Hash(requirementsFilePath)
	if err != nil {
		return true
	}

	return string(oldFileMD5HashBytes) != newFileMD5Hash
}

func writeMD5Hash(requirementsFile string, requirementsHashFile string) error {
	newFileMD5Hash, err := getMD5Hash(requirementsFile)
	if err != nil {
		return err
	}

	return os.WriteFile(requirementsHashFile, []byte(newFileMD5Hash), 0644)
}

type AnsibleApp struct {
	Logger     lib.Logger
	Playbook   *AnsiblePlaybook
	Template   db.Template
	Repository db.Repository
}

func (t *AnsibleApp) SetLogger(logger lib.Logger) lib.Logger {
	t.Logger = logger
	t.Playbook.Logger = logger
	return logger
}

func (t *AnsibleApp) Run(args []string, environmentVars *[]string, inputs map[string]string, cb func(*os.Process)) error {
	return t.Playbook.RunPlaybook(args, environmentVars, inputs, cb)
}

func (t *AnsibleApp) Log(msg string) {
	t.Logger.Log(msg)
}

func (t *AnsibleApp) InstallRequirements() error {
	if err := t.installCollectionsRequirements(); err != nil {
		return err
	}
	if err := t.installRolesRequirements(); err != nil {
		return err
	}
	return nil
}

func (t *AnsibleApp) getRepoPath() string {
	repo := GitRepository{
		Logger:     t.Logger,
		TemplateID: t.Template.ID,
		Repository: t.Repository,
		Client:     CreateDefaultGitClient(),
	}

	return repo.GetFullPath()
}

func (t *AnsibleApp) installRolesRequirements() error {
	requirementsFilePath := fmt.Sprintf("%s/roles/requirements.yml", t.getRepoPath())
	requirementsHashFilePath := fmt.Sprintf("%s.md5", requirementsFilePath)

	if _, err := os.Stat(requirementsFilePath); err != nil {
		t.Log("No roles/requirements.yml file found. Skip galaxy install process.\n")
		return nil
	}

	if hasRequirementsChanges(requirementsFilePath, requirementsHashFilePath) {
		if err := t.runGalaxy([]string{
			"role",
			"install",
			"-r",
			requirementsFilePath,
			"--force",
		}); err != nil {
			return err
		}
		if err := writeMD5Hash(requirementsFilePath, requirementsHashFilePath); err != nil {
			return err
		}
	} else {
		t.Log("roles/requirements.yml has no changes. Skip galaxy install process.\n")
	}

	return nil
}

func (t *AnsibleApp) GetPlaybookDir() string {
	playbookPath := path.Join(t.getRepoPath(), t.Template.Playbook)

	return path.Dir(playbookPath)
}

func (t *AnsibleApp) installCollectionsRequirements() error {
	requirementsFilePath := path.Join(t.GetPlaybookDir(), "collections", "requirements.yml")
	requirementsHashFilePath := fmt.Sprintf("%s.md5", requirementsFilePath)

	if _, err := os.Stat(requirementsFilePath); err != nil {
		t.Log("No collections/requirements.yml file found. Skip galaxy install process.\n")
		return nil
	}

	if hasRequirementsChanges(requirementsFilePath, requirementsHashFilePath) {
		if err := t.runGalaxy([]string{
			"collection",
			"install",
			"-r",
			requirementsFilePath,
			"--force",
		}); err != nil {
			return err
		}
		if err := writeMD5Hash(requirementsFilePath, requirementsHashFilePath); err != nil {
			return err
		}
	} else {
		t.Log("collections/requirements.yml has no changes. Skip galaxy install process.\n")
	}

	return nil
}

func (t *AnsibleApp) runGalaxy(args []string) error {
	return t.Playbook.RunGalaxy(args)
}
