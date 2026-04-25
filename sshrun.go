package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type durationValue struct {
	value time.Duration
	set   bool
}

type stringListFlag []string

func (d *durationValue) String() string {
	if !d.set {
		return ""
	}
	return d.value.String()
}

func (d *durationValue) Set(raw string) error {
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return err
	}
	d.value = parsed
	d.set = true
	return nil
}

func (s *stringListFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *stringListFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

type cliOptions struct {
	host          string
	port          string
	user          string
	password      string
	key           string
	keyPassphrase string
	knownHosts    string
	strictHostKey bool
	cmd           string
	cmdFile       string
	typeFlag      string
	localPath     string
	remotePath    string
	fileName      string
	targetPath    string
	tempPath      string
	mode          string
	plan          string
	direction     string
	recursive     bool
	deleteExtra   bool
	dryRun        bool
	conflict      string
	include       stringListFlag
	exclude       stringListFlag
	timeout       time.Duration
	cmdTimeout    durationValue
}

type deployPlan struct {
	Steps []deployStep `json:"steps"`
}

type deployStep struct {
	Name            string   `json:"name"`
	Type            string   `json:"type"`
	Cmd             string   `json:"cmd"`
	CmdFile         string   `json:"cmdfile"`
	LocalPath       string   `json:"local_path"`
	RemotePath      string   `json:"remote_path"`
	FileName        string   `json:"file_name"`
	TargetPath      string   `json:"target_path"`
	TempPath        string   `json:"temp_path"`
	Mode            string   `json:"mode"`
	Timeout         string   `json:"timeout"`
	ContinueOnError bool     `json:"continue_on_error"`
	Direction       string   `json:"direction"`
	Recursive       bool     `json:"recursive"`
	DeleteExtra     bool     `json:"delete"`
	DryRun          bool     `json:"dry_run"`
	Conflict        string   `json:"conflict"`
	Include         []string `json:"include"`
	Exclude         []string `json:"exclude"`
}

type commandResult struct {
	Output   []byte
	ExitCode int
	TimedOut bool
}

type syncTree struct {
	Exists    bool
	Root      string
	RootIsDir bool
	Files     map[string]fileState
	Dirs      map[string]struct{}
}

type fileState struct {
	Size    int64
	ModTime time.Time
}

func main() {
	options, err := parseCLI()
	if err != nil {
		log.Fatal(err)
	}

	client, err := dialSSH(options)
	if err != nil {
		log.Fatalf("Connection failed: %v", err)
	}
	defer client.Close()

	if err := run(options, client); err != nil {
		log.Fatal(err)
	}
}

func parseCLI() (*cliOptions, error) {
	var options cliOptions

	flag.StringVar(&options.host, "host", "", "SSH server IP address")
	flag.StringVar(&options.port, "port", "22", "SSH server port")
	flag.StringVar(&options.user, "user", "", "SSH username")
	flag.StringVar(&options.password, "password", "", "SSH password, support HEX_ prefixed hex encoding")
	flag.StringVar(&options.key, "key", "", "SSH private key path, support HEX_ prefixed hex encoding")
	flag.StringVar(&options.keyPassphrase, "keyPassphrase", "", "SSH private key passphrase, support HEX_ prefixed hex encoding")
	flag.StringVar(&options.knownHosts, "knownHosts", "", "Known hosts file path, support HEX_ prefixed hex encoding")
	flag.BoolVar(&options.strictHostKey, "strictHostKey", false, "Enable strict host key verification with known_hosts")
	flag.StringVar(&options.cmd, "cmd", "", "Command to execute, support HEX_ prefixed hex encoding")
	flag.StringVar(&options.cmdFile, "cmdfile", "", "Read commands from file")
	flag.StringVar(&options.typeFlag, "type", "cmd", "Function type: cmd, upload, download, mkdir, remove, chmod, move, upload_atomic, deploy, sync")
	flag.StringVar(&options.localPath, "localPath", "", "Local file path")
	flag.StringVar(&options.remotePath, "remotePath", "", "Remote file path")
	flag.StringVar(&options.fileName, "fileName", "", "Target file name for upload or download")
	flag.StringVar(&options.targetPath, "targetPath", "", "Target remote path for move")
	flag.StringVar(&options.tempPath, "tempPath", "", "Temporary remote path for upload_atomic")
	flag.StringVar(&options.mode, "mode", "", "Mode for chmod, e.g. 0755")
	flag.StringVar(&options.plan, "plan", "", "Deploy plan JSON file path")
	flag.StringVar(&options.direction, "direction", "push", "Sync direction: push, pull, bidirectional")
	flag.BoolVar(&options.recursive, "recursive", false, "Sync directories recursively")
	flag.BoolVar(&options.deleteExtra, "delete", false, "Delete extra files on sync target")
	flag.BoolVar(&options.dryRun, "dryRun", false, "Print planned sync actions without changing files")
	flag.StringVar(&options.conflict, "conflict", "fail_on_conflict", "Bidirectional conflict policy: fail_on_conflict, newer_wins, local_wins, remote_wins")
	flag.Var(&options.include, "include", "Include glob pattern for sync, can be repeated")
	flag.Var(&options.exclude, "exclude", "Exclude glob pattern for sync, can be repeated")
	flag.DurationVar(&options.timeout, "timeout", 30*time.Second, "SSH connection timeout")
	flag.Var(&options.cmdTimeout, "cmdTimeout", "Remote command timeout, e.g. 30s")
	flag.Parse()

	var err error
	options.host, err = decodeHexIfNeeded(options.host)
	if err != nil {
		return nil, fmt.Errorf("invalid host: %w", err)
	}
	options.port, err = decodeHexIfNeeded(options.port)
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}
	options.user, err = decodeHexIfNeeded(options.user)
	if err != nil {
		return nil, fmt.Errorf("invalid user: %w", err)
	}
	options.password, err = decodeHexIfNeeded(options.password)
	if err != nil {
		return nil, fmt.Errorf("invalid password: %w", err)
	}
	options.key, err = decodeHexIfNeeded(options.key)
	if err != nil {
		return nil, fmt.Errorf("invalid key: %w", err)
	}
	options.keyPassphrase, err = decodeHexIfNeeded(options.keyPassphrase)
	if err != nil {
		return nil, fmt.Errorf("invalid keyPassphrase: %w", err)
	}
	options.knownHosts, err = decodeHexIfNeeded(options.knownHosts)
	if err != nil {
		return nil, fmt.Errorf("invalid knownHosts: %w", err)
	}
	options.cmd, err = decodeHexIfNeeded(options.cmd)
	if err != nil {
		return nil, fmt.Errorf("invalid cmd: %w", err)
	}
	options.cmdFile, err = decodeHexIfNeeded(options.cmdFile)
	if err != nil {
		return nil, fmt.Errorf("invalid cmdfile: %w", err)
	}
	options.localPath, err = decodeHexIfNeeded(options.localPath)
	if err != nil {
		return nil, fmt.Errorf("invalid localPath: %w", err)
	}
	options.remotePath, err = decodeHexIfNeeded(options.remotePath)
	if err != nil {
		return nil, fmt.Errorf("invalid remotePath: %w", err)
	}
	options.fileName, err = decodeHexIfNeeded(options.fileName)
	if err != nil {
		return nil, fmt.Errorf("invalid fileName: %w", err)
	}
	options.targetPath, err = decodeHexIfNeeded(options.targetPath)
	if err != nil {
		return nil, fmt.Errorf("invalid targetPath: %w", err)
	}
	options.tempPath, err = decodeHexIfNeeded(options.tempPath)
	if err != nil {
		return nil, fmt.Errorf("invalid tempPath: %w", err)
	}
	options.mode, err = decodeHexIfNeeded(options.mode)
	if err != nil {
		return nil, fmt.Errorf("invalid mode: %w", err)
	}
	options.plan, err = decodeHexIfNeeded(options.plan)
	if err != nil {
		return nil, fmt.Errorf("invalid plan: %w", err)
	}
	options.include, err = decodeStringList(options.include)
	if err != nil {
		return nil, fmt.Errorf("invalid include: %w", err)
	}
	options.exclude, err = decodeStringList(options.exclude)
	if err != nil {
		return nil, fmt.Errorf("invalid exclude: %w", err)
	}

	if options.host == "" || options.user == "" {
		return nil, errors.New("must specify host and user parameters")
	}
	if options.password == "" && options.key == "" {
		return nil, errors.New("must specify password or key parameter")
	}

	return &options, nil
}

func run(options *cliOptions, client *ssh.Client) error {
	switch strings.ToLower(options.typeFlag) {
	case "cmd":
		commands, err := readCommands(options.cmd, options.cmdFile)
		if err != nil {
			return err
		}
		if len(commands) == 0 {
			return errors.New("must specify cmd or cmdfile parameter")
		}
		for _, cmd := range commands {
			result, err := runRemoteCommand(client, cmd, options.cmdTimeout.value)
			if len(result.Output) > 0 {
				fmt.Printf("%s", result.Output)
			}
			if err != nil {
				return fmt.Errorf("failed to execute command %q: %w", cmd, err)
			}
		}
		return nil

	case "upload":
		sftpClient, err := newSFTPClient(client)
		if err != nil {
			return err
		}
		defer sftpClient.Close()
		_, err = uploadFile(sftpClient, options.localPath, options.remotePath, options.fileName)
		return err

	case "download":
		sftpClient, err := newSFTPClient(client)
		if err != nil {
			return err
		}
		defer sftpClient.Close()
		_, err = downloadFile(sftpClient, options.localPath, options.remotePath, options.fileName)
		return err

	case "mkdir":
		if options.remotePath == "" {
			return errors.New("must specify remotePath parameter for mkdir")
		}
		sftpClient, err := newSFTPClient(client)
		if err != nil {
			return err
		}
		defer sftpClient.Close()
		return makeRemoteDir(sftpClient, options.remotePath)

	case "remove":
		if options.remotePath == "" {
			return errors.New("must specify remotePath parameter for remove")
		}
		sftpClient, err := newSFTPClient(client)
		if err != nil {
			return err
		}
		defer sftpClient.Close()
		return removeRemotePath(sftpClient, options.remotePath)

	case "chmod":
		mode := options.mode
		if mode == "" {
			mode = options.fileName
		}
		if options.remotePath == "" || mode == "" {
			return errors.New("must specify remotePath and mode parameters for chmod")
		}
		sftpClient, err := newSFTPClient(client)
		if err != nil {
			return err
		}
		defer sftpClient.Close()
		return chmodRemotePath(sftpClient, options.remotePath, mode)

	case "move":
		targetPath := options.targetPath
		if targetPath == "" {
			targetPath = options.localPath
		}
		if options.remotePath == "" || targetPath == "" {
			return errors.New("must specify remotePath and targetPath parameters for move")
		}
		sftpClient, err := newSFTPClient(client)
		if err != nil {
			return err
		}
		defer sftpClient.Close()
		return moveRemotePath(sftpClient, options.remotePath, targetPath)

	case "upload_atomic":
		tempPath := options.tempPath
		if tempPath == "" {
			tempPath = options.fileName
		}
		sftpClient, err := newSFTPClient(client)
		if err != nil {
			return err
		}
		defer sftpClient.Close()
		_, err = uploadAtomicFile(sftpClient, options.localPath, options.remotePath, tempPath)
		return err

	case "deploy":
		if options.plan == "" {
			return errors.New("must specify plan parameter for deploy")
		}
		sftpClient, err := newSFTPClient(client)
		if err != nil {
			return err
		}
		defer sftpClient.Close()
		return runDeployPlan(client, sftpClient, options.plan, options.cmdTimeout.value)

	case "sync":
		if options.localPath == "" || options.remotePath == "" {
			return errors.New("must specify localPath and remotePath parameters for sync")
		}
		sftpClient, err := newSFTPClient(client)
		if err != nil {
			return err
		}
		defer sftpClient.Close()
		return runSync(sftpClient, options.localPath, options.remotePath, options.direction, options.recursive, options.deleteExtra, options.dryRun, options.conflict, options.include, options.exclude)

	default:
		return fmt.Errorf("invalid function type: %s, optional values are cmd, upload, download, mkdir, remove, chmod, move, upload_atomic, deploy, sync", options.typeFlag)
	}
}

func newSFTPClient(client *ssh.Client) (*sftp.Client, error) {
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}
	return sftpClient, nil
}

func dialSSH(options *cliOptions) (*ssh.Client, error) {
	authMethods, err := buildAuthMethods(options.password, options.key, options.keyPassphrase)
	if err != nil {
		return nil, err
	}

	hostKeyCallback, err := buildHostKeyCallback(options.strictHostKey, options.knownHosts)
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User:            options.user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         options.timeout,
	}

	addr := net.JoinHostPort(options.host, options.port)
	return ssh.Dial("tcp", addr, config)
}

func buildAuthMethods(password, keyPath, keyPassphrase string) ([]ssh.AuthMethod, error) {
	authMethods := make([]ssh.AuthMethod, 0, 2)
	if password != "" {
		authMethods = append(authMethods, ssh.Password(password))
	}
	if keyPath != "" {
		signer, err := loadPrivateKey(keyPath, keyPassphrase)
		if err != nil {
			return nil, err
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	if len(authMethods) == 0 {
		return nil, errors.New("no SSH auth method configured")
	}
	return authMethods, nil
}

func loadPrivateKey(keyPath, keyPassphrase string) (ssh.Signer, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read key file: %w", err)
	}
	if keyPassphrase == "" {
		signer, err := ssh.ParsePrivateKey(data)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		return signer, nil
	}
	signer, err := ssh.ParsePrivateKeyWithPassphrase(data, []byte(keyPassphrase))
	if err != nil {
		return nil, fmt.Errorf("parse private key with passphrase: %w", err)
	}
	return signer, nil
}

func buildHostKeyCallback(strict bool, knownHostsPath string) (ssh.HostKeyCallback, error) {
	if !strict {
		return ssh.InsecureIgnoreHostKey(), nil
	}
	if knownHostsPath == "" {
		return nil, errors.New("must specify knownHosts when strictHostKey is enabled")
	}
	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("load known hosts: %w", err)
	}
	return callback, nil
}

func readCommands(command, commandFile string) ([]string, error) {
	if commandFile != "" {
		data, err := os.ReadFile(commandFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read command file: %w", err)
		}
		lines := strings.Split(string(data), "\n")
		commands := make([]string, 0, len(lines))
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				commands = append(commands, line)
			}
		}
		return commands, nil
	}
	if command == "" {
		return nil, nil
	}
	return []string{command}, nil
}

func runRemoteCommand(client *ssh.Client, cmd string, timeout time.Duration) (commandResult, error) {
	var result commandResult

	session, err := client.NewSession()
	if err != nil {
		return result, fmt.Errorf("create session: %w", err)
	}
	defer session.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if timeout <= 0 {
		if err := session.Run(cmd); err != nil {
			result.Output = append(stdout.Bytes(), stderr.Bytes()...)
			return result, fmt.Errorf("run command: %w", err)
		}
		result.Output = append(stdout.Bytes(), stderr.Bytes()...)
		return result, nil
	}

	if err := session.Start(cmd); err != nil {
		return result, fmt.Errorf("start command: %w", err)
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- session.Wait()
	}()

	select {
	case err := <-waitCh:
		result.Output = append(stdout.Bytes(), stderr.Bytes()...)
		if err != nil {
			return result, fmt.Errorf("run command: %w", err)
		}
		return result, nil
	case <-time.After(timeout):
		result.Output = append(stdout.Bytes(), stderr.Bytes()...)
		result.TimedOut = true
		_ = session.Close()
		return result, fmt.Errorf("command timed out after %s", timeout)
	}
}

func uploadFile(sftpClient *sftp.Client, localPath, remotePath, fileName string) (int64, error) {
	if localPath == "" || remotePath == "" {
		return 0, errors.New("must specify localPath and remotePath parameters for upload")
	}

	remoteFile := resolveRemoteUploadPath(localPath, remotePath, fileName)
	if err := sftpClient.MkdirAll(path.Dir(remoteFile)); err != nil {
		return 0, fmt.Errorf("create remote parent dir: %w", err)
	}

	localFile, err := os.Open(localPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	remoteHandle, err := sftpClient.Create(remoteFile)
	if err != nil {
		return 0, fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteHandle.Close()

	fmt.Printf("Uploading %s to %s...\n", localPath, remoteFile)
	written, err := io.Copy(remoteHandle, localFile)
	if err != nil {
		return written, fmt.Errorf("failed to write remote file: %w", err)
	}
	fmt.Printf("Successfully uploaded %d bytes to %s\n", written, remoteFile)
	return written, nil
}

func uploadAtomicFile(sftpClient *sftp.Client, localPath, remotePath, tempPath string) (int64, error) {
	if localPath == "" || remotePath == "" {
		return 0, errors.New("must specify localPath and remotePath parameters for upload_atomic")
	}
	if tempPath == "" {
		tempPath = remotePath + ".tmp"
	}
	if _, err := uploadFile(sftpClient, localPath, tempPath, ""); err != nil {
		return 0, err
	}
	if err := moveRemotePath(sftpClient, tempPath, remotePath); err != nil {
		return 0, err
	}
	info, err := sftpClient.Stat(remotePath)
	if err != nil {
		return 0, fmt.Errorf("stat uploaded file: %w", err)
	}
	return info.Size(), nil
}

func downloadFile(sftpClient *sftp.Client, localPath, remotePath, fileName string) (int64, error) {
	if localPath == "" || remotePath == "" {
		return 0, errors.New("must specify localPath and remotePath parameters for download")
	}

	localFile := resolveLocalDownloadPath(localPath, remotePath, fileName)
	if err := os.MkdirAll(filepath.Dir(localFile), 0755); err != nil {
		return 0, fmt.Errorf("failed to create local directory: %w", err)
	}

	remoteHandle, err := sftpClient.Open(remotePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open remote file: %w", err)
	}
	defer remoteHandle.Close()

	localHandle, err := os.Create(localFile)
	if err != nil {
		return 0, fmt.Errorf("failed to create local file: %w", err)
	}
	defer localHandle.Close()

	written, err := io.Copy(localHandle, remoteHandle)
	if err != nil {
		return written, fmt.Errorf("failed to read remote file: %w", err)
	}
	fmt.Printf("Successfully downloaded %d bytes to %s\n", written, localFile)
	return written, nil
}

func makeRemoteDir(sftpClient *sftp.Client, remotePath string) error {
	if err := sftpClient.MkdirAll(remotePath); err != nil {
		return fmt.Errorf("failed to create remote directory %s: %w", remotePath, err)
	}
	fmt.Printf("Successfully created remote directory %s\n", remotePath)
	return nil
}

func removeRemotePath(sftpClient *sftp.Client, remotePath string) error {
	info, err := sftpClient.Stat(remotePath)
	if err != nil {
		if isNotExistError(err) {
			fmt.Printf("Remote path %s does not exist, skipping\n", remotePath)
			return nil
		}
		return fmt.Errorf("stat remote path %s: %w", remotePath, err)
	}
	if !info.IsDir() {
		if err := sftpClient.Remove(remotePath); err != nil {
			return fmt.Errorf("remove remote file %s: %w", remotePath, err)
		}
		fmt.Printf("Successfully removed remote file %s\n", remotePath)
		return nil
	}
	if err := removeRemoteTree(sftpClient, remotePath); err != nil {
		return err
	}
	fmt.Printf("Successfully removed remote directory %s\n", remotePath)
	return nil
}

func removeRemoteTree(sftpClient *sftp.Client, remotePath string) error {
	entries, err := sftpClient.ReadDir(remotePath)
	if err != nil {
		return fmt.Errorf("read remote dir %s: %w", remotePath, err)
	}
	for _, entry := range entries {
		child := path.Join(remotePath, entry.Name())
		if entry.IsDir() {
			if err := removeRemoteTree(sftpClient, child); err != nil {
				return err
			}
			continue
		}
		if err := sftpClient.Remove(child); err != nil {
			return fmt.Errorf("remove remote file %s: %w", child, err)
		}
	}
	if err := sftpClient.RemoveDirectory(remotePath); err != nil {
		return fmt.Errorf("remove remote directory %s: %w", remotePath, err)
	}
	return nil
}

func chmodRemotePath(sftpClient *sftp.Client, remotePath, mode string) error {
	parsed, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return fmt.Errorf("parse chmod mode %q: %w", mode, err)
	}
	if err := sftpClient.Chmod(remotePath, os.FileMode(parsed)); err != nil {
		return fmt.Errorf("chmod remote path %s: %w", remotePath, err)
	}
	fmt.Printf("Successfully changed mode for %s to %s\n", remotePath, mode)
	return nil
}

func moveRemotePath(sftpClient *sftp.Client, sourcePath, targetPath string) error {
	if err := sftpClient.MkdirAll(path.Dir(targetPath)); err != nil {
		return fmt.Errorf("create target parent dir: %w", err)
	}
	_ = sftpClient.Remove(targetPath)
	if err := sftpClient.PosixRename(sourcePath, targetPath); err != nil {
		if err := sftpClient.Rename(sourcePath, targetPath); err != nil {
			return fmt.Errorf("move remote path %s -> %s: %w", sourcePath, targetPath, err)
		}
	}
	fmt.Printf("Successfully moved %s to %s\n", sourcePath, targetPath)
	return nil
}

func runDeployPlan(client *ssh.Client, sftpClient *sftp.Client, planPath string, defaultTimeout time.Duration) error {
	plan, err := loadDeployPlan(planPath)
	if err != nil {
		return err
	}
	if len(plan.Steps) == 0 {
		return errors.New("deploy plan must contain at least one step")
	}

	for idx, step := range plan.Steps {
		label := step.Name
		if label == "" {
			label = fmt.Sprintf("step-%d", idx+1)
		}
		fmt.Printf("==> [%d/%d] %s (%s)\n", idx+1, len(plan.Steps), label, step.Type)
		if err := executeDeployStep(client, sftpClient, step, defaultTimeout); err != nil {
			if step.ContinueOnError {
				fmt.Printf("Step failed but continue_on_error=true: %v\n", err)
				continue
			}
			return fmt.Errorf("deploy step %q failed: %w", label, err)
		}
	}

	return nil
}

func loadDeployPlan(planPath string) (*deployPlan, error) {
	data, err := os.ReadFile(planPath)
	if err != nil {
		return nil, fmt.Errorf("read deploy plan: %w", err)
	}
	var plan deployPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("parse deploy plan: %w", err)
	}
	return &plan, nil
}

func executeDeployStep(client *ssh.Client, sftpClient *sftp.Client, step deployStep, defaultTimeout time.Duration) error {
	typeName := strings.ToLower(step.Type)
	stepTimeout, err := resolveStepTimeout(step.Timeout, defaultTimeout)
	if err != nil {
		return err
	}

	switch typeName {
	case "cmd":
		commands, err := readCommands(step.Cmd, step.CmdFile)
		if err != nil {
			return err
		}
		if len(commands) == 0 {
			return errors.New("cmd step requires cmd or cmdfile")
		}
		for _, cmd := range commands {
			result, err := runRemoteCommand(client, cmd, stepTimeout)
			if len(result.Output) > 0 {
				fmt.Printf("%s", result.Output)
			}
			if err != nil {
				return err
			}
		}
		return nil

	case "upload":
		_, err := uploadFile(sftpClient, step.LocalPath, step.RemotePath, step.FileName)
		return err

	case "download":
		_, err := downloadFile(sftpClient, step.LocalPath, step.RemotePath, step.FileName)
		return err

	case "mkdir":
		return makeRemoteDir(sftpClient, step.RemotePath)

	case "remove":
		return removeRemotePath(sftpClient, step.RemotePath)

	case "chmod":
		return chmodRemotePath(sftpClient, step.RemotePath, step.Mode)

	case "move":
		return moveRemotePath(sftpClient, step.RemotePath, step.TargetPath)

	case "upload_atomic":
		_, err := uploadAtomicFile(sftpClient, step.LocalPath, step.RemotePath, step.TempPath)
		return err

	case "sync":
		direction := step.Direction
		if direction == "" {
			direction = "push"
		}
		conflict := step.Conflict
		if conflict == "" {
			conflict = "fail_on_conflict"
		}
		return runSync(sftpClient, step.LocalPath, step.RemotePath, direction, step.Recursive, step.DeleteExtra, step.DryRun, conflict, step.Include, step.Exclude)

	default:
		return fmt.Errorf("unsupported deploy step type: %s", step.Type)
	}
}

func runSync(sftpClient *sftp.Client, localPath, remotePath, direction string, recursive, deleteExtra, dryRun bool, conflict string, includePatterns, excludePatterns []string) error {
	direction = strings.ToLower(strings.TrimSpace(direction))
	conflict = strings.ToLower(strings.TrimSpace(conflict))
	if direction == "" {
		direction = "push"
	}
	if conflict == "" {
		conflict = "fail_on_conflict"
	}

	localInfo, localExists, err := statLocalPath(localPath)
	if err != nil {
		return err
	}
	remoteInfo, remoteExists, err := statRemotePath(sftpClient, remotePath)
	if err != nil {
		return err
	}

	switch direction {
	case "push":
		if !localExists {
			return fmt.Errorf("local path does not exist: %s", localPath)
		}
		if localInfo.IsDir() {
			return syncDirectoryPush(sftpClient, localPath, remotePath, recursive, deleteExtra, dryRun, includePatterns, excludePatterns)
		}
		return syncFilePush(sftpClient, localPath, remotePath, remoteInfo, remoteExists, dryRun)

	case "pull":
		if !remoteExists {
			return fmt.Errorf("remote path does not exist: %s", remotePath)
		}
		if remoteInfo.IsDir() {
			return syncDirectoryPull(sftpClient, localPath, remotePath, recursive, deleteExtra, dryRun, includePatterns, excludePatterns)
		}
		return syncFilePull(sftpClient, localPath, remotePath, localInfo, localExists, dryRun)

	case "bidirectional":
		if deleteExtra {
			return errors.New("-delete is not supported for bidirectional sync")
		}
		kind, err := detectBidirectionalKind(localInfo, localExists, remoteInfo, remoteExists)
		if err != nil {
			return err
		}
		if kind == "dir" {
			return syncDirectoryBidirectional(sftpClient, localPath, remotePath, recursive, dryRun, conflict, includePatterns, excludePatterns)
		}
		return syncFileBidirectional(sftpClient, localPath, remotePath, localInfo, localExists, remoteInfo, remoteExists, dryRun, conflict)

	default:
		return fmt.Errorf("invalid sync direction: %s, optional values are push, pull, bidirectional", direction)
	}
}

func syncDirectoryPush(sftpClient *sftp.Client, localPath, remotePath string, recursive, deleteExtra, dryRun bool, includePatterns, excludePatterns []string) error {
	source, err := collectLocalTree(localPath, recursive, includePatterns, excludePatterns)
	if err != nil {
		return err
	}
	target, err := collectRemoteTree(sftpClient, remotePath, recursive, true, includePatterns, excludePatterns)
	if err != nil {
		return err
	}

	if !dryRun {
		if err := ensureRemoteRootDir(sftpClient, remotePath, target.Exists, target.RootIsDir); err != nil {
			return err
		}
	} else {
		fmt.Printf("[dry-run] ensure remote directory %s\n", remotePath)
	}

	return syncTreePush(sftpClient, source, target, remotePath, deleteExtra, dryRun)
}

func syncDirectoryPull(sftpClient *sftp.Client, localPath, remotePath string, recursive, deleteExtra, dryRun bool, includePatterns, excludePatterns []string) error {
	source, err := collectRemoteTree(sftpClient, remotePath, recursive, false, includePatterns, excludePatterns)
	if err != nil {
		return err
	}
	target, err := collectLocalTreeAllowMissing(localPath, recursive, includePatterns, excludePatterns)
	if err != nil {
		return err
	}

	if !dryRun {
		if err := ensureLocalRootDir(localPath, target.Exists, target.RootIsDir); err != nil {
			return err
		}
	} else {
		fmt.Printf("[dry-run] ensure local directory %s\n", localPath)
	}

	return syncTreePull(sftpClient, source, target, localPath, deleteExtra, dryRun)
}

func syncDirectoryBidirectional(sftpClient *sftp.Client, localPath, remotePath string, recursive, dryRun bool, conflict string, includePatterns, excludePatterns []string) error {
	localTree, err := collectLocalTreeAllowMissing(localPath, recursive, includePatterns, excludePatterns)
	if err != nil {
		return err
	}
	remoteTree, err := collectRemoteTree(sftpClient, remotePath, recursive, true, includePatterns, excludePatterns)
	if err != nil {
		return err
	}

	if !localTree.Exists && !remoteTree.Exists {
		return fmt.Errorf("both paths are missing: %s and %s", localPath, remotePath)
	}
	if !dryRun {
		if localTree.Exists {
			if !localTree.RootIsDir {
				return fmt.Errorf("local path is not a directory: %s", localPath)
			}
		} else if err := os.MkdirAll(localPath, 0755); err != nil {
			return fmt.Errorf("create local root dir: %w", err)
		}
		if remoteTree.Exists {
			if !remoteTree.RootIsDir {
				return fmt.Errorf("remote path is not a directory: %s", remotePath)
			}
		} else if err := sftpClient.MkdirAll(remotePath); err != nil {
			return fmt.Errorf("create remote root dir: %w", err)
		}
	} else {
		fmt.Printf("[dry-run] ensure bidirectional roots %s <-> %s\n", localPath, remotePath)
	}

	return syncTreeBidirectional(sftpClient, localPath, remotePath, localTree, remoteTree, dryRun, conflict)
}

func syncTreePush(sftpClient *sftp.Client, source, target *syncTree, remoteRoot string, deleteExtra, dryRun bool) error {
	dirs := sortedDirs(source.Dirs)
	for _, rel := range dirs {
		remoteDir := joinRemoteRoot(remoteRoot, rel)
		if dryRun {
			fmt.Printf("[dry-run] mkdir %s\n", remoteDir)
			continue
		}
		if err := sftpClient.MkdirAll(remoteDir); err != nil {
			return fmt.Errorf("create remote directory %s: %w", remoteDir, err)
		}
	}

	for rel, state := range source.Files {
		if targetState, ok := target.Files[rel]; ok && sameFileState(state, targetState) {
			continue
		}
		localFile := joinLocalRoot(source.Root, rel)
		remoteFile := joinRemoteRoot(remoteRoot, rel)
		if dryRun {
			fmt.Printf("[dry-run] upload %s -> %s\n", localFile, remoteFile)
			continue
		}
		if _, err := uploadFilePreserveTime(sftpClient, localFile, remoteFile); err != nil {
			return err
		}
	}

	if deleteExtra {
		if err := deleteExtraRemoteEntries(sftpClient, remoteRoot, source, target, dryRun); err != nil {
			return err
		}
	}

	return nil
}

func syncTreePull(sftpClient *sftp.Client, source, target *syncTree, localRoot string, deleteExtra, dryRun bool) error {
	dirs := sortedDirs(source.Dirs)
	for _, rel := range dirs {
		localDir := joinLocalRoot(localRoot, rel)
		if dryRun {
			fmt.Printf("[dry-run] mkdir %s\n", localDir)
			continue
		}
		if err := os.MkdirAll(localDir, 0755); err != nil {
			return fmt.Errorf("create local directory %s: %w", localDir, err)
		}
	}

	for rel, state := range source.Files {
		if targetState, ok := target.Files[rel]; ok && sameFileState(state, targetState) {
			continue
		}
		remoteFile := joinRemoteRoot(source.Root, rel)
		localFile := joinLocalRoot(localRoot, rel)
		if dryRun {
			fmt.Printf("[dry-run] download %s -> %s\n", remoteFile, localFile)
			continue
		}
		if _, err := downloadFilePreserveTime(sftpClient, localFile, remoteFile); err != nil {
			return err
		}
	}

	if deleteExtra {
		if err := deleteExtraLocalEntries(localRoot, source, target, dryRun); err != nil {
			return err
		}
	}

	return nil
}

func syncTreeBidirectional(sftpClient *sftp.Client, localRoot, remoteRoot string, localTree, remoteTree *syncTree, dryRun bool, conflict string) error {
	allDirs := mergeDirKeys(localTree.Dirs, remoteTree.Dirs)
	for _, rel := range allDirs {
		localDir := joinLocalRoot(localRoot, rel)
		remoteDir := joinRemoteRoot(remoteRoot, rel)
		if dryRun {
			if _, ok := localTree.Dirs[rel]; !ok {
				fmt.Printf("[dry-run] mkdir %s\n", localDir)
			}
			if _, ok := remoteTree.Dirs[rel]; !ok {
				fmt.Printf("[dry-run] mkdir %s\n", remoteDir)
			}
			continue
		}
		if _, ok := localTree.Dirs[rel]; !ok {
			if err := os.MkdirAll(localDir, 0755); err != nil {
				return fmt.Errorf("create local directory %s: %w", localDir, err)
			}
		}
		if _, ok := remoteTree.Dirs[rel]; !ok {
			if err := sftpClient.MkdirAll(remoteDir); err != nil {
				return fmt.Errorf("create remote directory %s: %w", remoteDir, err)
			}
		}
	}

	allFiles := mergeFileKeys(localTree.Files, remoteTree.Files)
	for _, rel := range allFiles {
		localState, localOK := localTree.Files[rel]
		remoteState, remoteOK := remoteTree.Files[rel]
		localFile := joinLocalRoot(localRoot, rel)
		remoteFile := joinRemoteRoot(remoteRoot, rel)

		switch {
		case localOK && !remoteOK:
			if dryRun {
				fmt.Printf("[dry-run] upload %s -> %s\n", localFile, remoteFile)
				continue
			}
			if _, err := uploadFilePreserveTime(sftpClient, localFile, remoteFile); err != nil {
				return err
			}

		case !localOK && remoteOK:
			if dryRun {
				fmt.Printf("[dry-run] download %s -> %s\n", remoteFile, localFile)
				continue
			}
			if _, err := downloadFilePreserveTime(sftpClient, localFile, remoteFile); err != nil {
				return err
			}

		case sameFileState(localState, remoteState):
			continue

		default:
			action, err := resolveBidirectionalAction(localState, remoteState, conflict)
			if err != nil {
				return fmt.Errorf("conflict on %s: %w", rel, err)
			}
			switch action {
			case "upload":
				if dryRun {
					fmt.Printf("[dry-run] upload %s -> %s\n", localFile, remoteFile)
					continue
				}
				if _, err := uploadFilePreserveTime(sftpClient, localFile, remoteFile); err != nil {
					return err
				}
			case "download":
				if dryRun {
					fmt.Printf("[dry-run] download %s -> %s\n", remoteFile, localFile)
					continue
				}
				if _, err := downloadFilePreserveTime(sftpClient, localFile, remoteFile); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unsupported bidirectional action: %s", action)
			}
		}
	}

	return nil
}

func syncFilePush(sftpClient *sftp.Client, localPath, remotePath string, remoteInfo os.FileInfo, remoteExists, dryRun bool) error {
	resolvedRemotePath := resolveRemoteSyncPath(localPath, remotePath, remoteInfo, remoteExists)
	localInfo, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("stat local file %s: %w", localPath, err)
	}
	resolvedRemoteInfo, resolvedRemoteExists, err := statRemotePath(sftpClient, resolvedRemotePath)
	if err != nil {
		return err
	}
	if resolvedRemoteExists && sameFileInfo(localInfo, resolvedRemoteInfo) {
		return nil
	}
	if dryRun {
		fmt.Printf("[dry-run] upload %s -> %s\n", localPath, resolvedRemotePath)
		return nil
	}
	_, err = uploadFilePreserveTime(sftpClient, localPath, resolvedRemotePath)
	return err
}

func syncFilePull(sftpClient *sftp.Client, localPath, remotePath string, localInfo os.FileInfo, localExists, dryRun bool) error {
	resolvedLocalPath := resolveLocalSyncPath(localPath, remotePath, localInfo, localExists)
	resolvedLocalInfo, resolvedLocalExists, err := statLocalPath(resolvedLocalPath)
	if err != nil {
		return err
	}
	remoteInfo, remoteExists, err := statRemotePath(sftpClient, remotePath)
	if err != nil {
		return err
	}
	if !remoteExists {
		return fmt.Errorf("remote file does not exist: %s", remotePath)
	}
	if resolvedLocalExists && sameFileInfo(resolvedLocalInfo, remoteInfo) {
		return nil
	}
	if dryRun {
		fmt.Printf("[dry-run] download %s -> %s\n", remotePath, resolvedLocalPath)
		return nil
	}
	_, err = downloadFilePreserveTime(sftpClient, resolvedLocalPath, remotePath)
	return err
}

func syncFileBidirectional(sftpClient *sftp.Client, localPath, remotePath string, localInfo os.FileInfo, localExists bool, remoteInfo os.FileInfo, remoteExists, dryRun bool, conflict string) error {
	if !localExists && !remoteExists {
		return fmt.Errorf("both files are missing: %s and %s", localPath, remotePath)
	}
	if localExists && !remoteExists {
		return syncFilePush(sftpClient, localPath, remotePath, nil, false, dryRun)
	}
	if !localExists && remoteExists {
		return syncFilePull(sftpClient, localPath, remotePath, nil, false, dryRun)
	}
	if sameFileInfo(localInfo, remoteInfo) {
		return nil
	}
	localState := fileState{Size: localInfo.Size(), ModTime: localInfo.ModTime()}
	remoteState := fileState{Size: remoteInfo.Size(), ModTime: remoteInfo.ModTime()}
	action, err := resolveBidirectionalAction(localState, remoteState, conflict)
	if err != nil {
		return err
	}
	if action == "upload" {
		return syncFilePush(sftpClient, localPath, remotePath, remoteInfo, true, dryRun)
	}
	return syncFilePull(sftpClient, localPath, remotePath, localInfo, true, dryRun)
}

func collectLocalTree(root string, recursive bool, includePatterns, excludePatterns []string) (*syncTree, error) {
	tree, err := collectLocalTreeAllowMissing(root, recursive, includePatterns, excludePatterns)
	if err != nil {
		return nil, err
	}
	if !tree.Exists {
		return nil, fmt.Errorf("local path does not exist: %s", root)
	}
	return tree, nil
}

func collectLocalTreeAllowMissing(root string, recursive bool, includePatterns, excludePatterns []string) (*syncTree, error) {
	info, exists, err := statLocalPath(root)
	if err != nil {
		return nil, err
	}
	tree := &syncTree{Exists: exists, Root: root, Files: map[string]fileState{}, Dirs: map[string]struct{}{}}
	if !exists {
		return tree, nil
	}
	tree.RootIsDir = info.IsDir()
	if !info.IsDir() {
		tree.Files[""] = fileState{Size: info.Size(), ModTime: info.ModTime()}
		return tree, nil
	}
	if !recursive {
		return nil, fmt.Errorf("directory sync requires -recursive=true: %s", root)
	}
	err = filepath.Walk(root, func(current string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == root {
			return nil
		}
		rel, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !shouldSyncPath(rel, info.IsDir(), includePatterns, excludePatterns) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			tree.Dirs[rel] = struct{}{}
			return nil
		}
		tree.Files[rel] = fileState{Size: info.Size(), ModTime: info.ModTime()}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan local tree %s: %w", root, err)
	}
	addParentDirs(tree)
	return tree, nil
}

func collectRemoteTree(sftpClient *sftp.Client, root string, recursive, allowMissing bool, includePatterns, excludePatterns []string) (*syncTree, error) {
	info, exists, err := statRemotePath(sftpClient, root)
	if err != nil {
		return nil, err
	}
	tree := &syncTree{Exists: exists, Root: root, Files: map[string]fileState{}, Dirs: map[string]struct{}{}}
	if !exists {
		if allowMissing {
			return tree, nil
		}
		return nil, fmt.Errorf("remote path does not exist: %s", root)
	}
	tree.RootIsDir = info.IsDir()
	if !info.IsDir() {
		tree.Files[""] = fileState{Size: info.Size(), ModTime: info.ModTime()}
		return tree, nil
	}
	if !recursive {
		return nil, fmt.Errorf("directory sync requires -recursive=true: %s", root)
	}
	if err := walkRemoteTree(sftpClient, root, "", tree, includePatterns, excludePatterns); err != nil {
		return nil, err
	}
	addParentDirs(tree)
	return tree, nil
}

func walkRemoteTree(sftpClient *sftp.Client, root, rel string, tree *syncTree, includePatterns, excludePatterns []string) error {
	remotePath := root
	if rel != "" {
		remotePath = path.Join(root, rel)
	}
	entries, err := sftpClient.ReadDir(remotePath)
	if err != nil {
		return fmt.Errorf("read remote directory %s: %w", remotePath, err)
	}
	for _, entry := range entries {
		entryRel := entry.Name()
		if rel != "" {
			entryRel = rel + "/" + entry.Name()
		}
		if !shouldSyncPath(entryRel, entry.IsDir(), includePatterns, excludePatterns) {
			continue
		}
		if entry.IsDir() {
			tree.Dirs[entryRel] = struct{}{}
			if err := walkRemoteTree(sftpClient, root, entryRel, tree, includePatterns, excludePatterns); err != nil {
				return err
			}
			continue
		}
		tree.Files[entryRel] = fileState{Size: entry.Size(), ModTime: entry.ModTime()}
	}
	return nil
}

func deleteExtraRemoteEntries(sftpClient *sftp.Client, remoteRoot string, source, target *syncTree, dryRun bool) error {
	for rel := range target.Files {
		if _, ok := source.Files[rel]; ok {
			continue
		}
		remotePath := joinRemoteRoot(remoteRoot, rel)
		if dryRun {
			fmt.Printf("[dry-run] remove %s\n", remotePath)
			continue
		}
		if err := sftpClient.Remove(remotePath); err != nil {
			return fmt.Errorf("remove remote file %s: %w", remotePath, err)
		}
	}
	for _, rel := range reverseSortedDirs(target.Dirs) {
		if _, ok := source.Dirs[rel]; ok {
			continue
		}
		remotePath := joinRemoteRoot(remoteRoot, rel)
		if dryRun {
			fmt.Printf("[dry-run] remove %s\n", remotePath)
			continue
		}
		if err := sftpClient.RemoveDirectory(remotePath); err != nil && !isNotExistError(err) {
			return fmt.Errorf("remove remote directory %s: %w", remotePath, err)
		}
	}
	return nil
}

func deleteExtraLocalEntries(localRoot string, source, target *syncTree, dryRun bool) error {
	for rel := range target.Files {
		if _, ok := source.Files[rel]; ok {
			continue
		}
		localPath := joinLocalRoot(localRoot, rel)
		if dryRun {
			fmt.Printf("[dry-run] remove %s\n", localPath)
			continue
		}
		if err := os.Remove(localPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove local file %s: %w", localPath, err)
		}
	}
	for _, rel := range reverseSortedDirs(target.Dirs) {
		if _, ok := source.Dirs[rel]; ok {
			continue
		}
		localPath := joinLocalRoot(localRoot, rel)
		if dryRun {
			fmt.Printf("[dry-run] remove %s\n", localPath)
			continue
		}
		if err := os.Remove(localPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove local directory %s: %w", localPath, err)
		}
	}
	return nil
}

func ensureRemoteRootDir(sftpClient *sftp.Client, remotePath string, exists, isDir bool) error {
	if exists {
		if !isDir {
			return fmt.Errorf("remote path is not a directory: %s", remotePath)
		}
		return nil
	}
	if err := sftpClient.MkdirAll(remotePath); err != nil {
		return fmt.Errorf("create remote root dir %s: %w", remotePath, err)
	}
	return nil
}

func ensureLocalRootDir(localPath string, exists, isDir bool) error {
	if exists {
		if !isDir {
			return fmt.Errorf("local path is not a directory: %s", localPath)
		}
		return nil
	}
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return fmt.Errorf("create local root dir %s: %w", localPath, err)
	}
	return nil
}

func resolveBidirectionalAction(localState, remoteState fileState, conflict string) (string, error) {
	switch conflict {
	case "local_wins":
		return "upload", nil
	case "remote_wins":
		return "download", nil
	case "newer_wins":
		if localState.ModTime.After(remoteState.ModTime) {
			return "upload", nil
		}
		if remoteState.ModTime.After(localState.ModTime) {
			return "download", nil
		}
		return "", fmt.Errorf("same modification time but different content")
	case "fail_on_conflict":
		return "", fmt.Errorf("local and remote versions differ")
	default:
		return "", fmt.Errorf("invalid conflict policy: %s", conflict)
	}
}

func uploadFilePreserveTime(sftpClient *sftp.Client, localPath, remotePath string) (int64, error) {
	written, err := uploadFile(sftpClient, localPath, remotePath, "")
	if err != nil {
		return written, err
	}
	info, statErr := os.Stat(localPath)
	if statErr == nil {
		if err := sftpClient.Chtimes(remotePath, info.ModTime(), info.ModTime()); err != nil {
			fmt.Printf("Warning: failed to preserve remote mtime for %s: %v\n", remotePath, err)
		}
	}
	return written, nil
}

func downloadFilePreserveTime(sftpClient *sftp.Client, localPath, remotePath string) (int64, error) {
	written, err := downloadFile(sftpClient, localPath, remotePath, "")
	if err != nil {
		return written, err
	}
	info, statErr := sftpClient.Stat(remotePath)
	if statErr == nil {
		if err := os.Chtimes(localPath, info.ModTime(), info.ModTime()); err != nil {
			fmt.Printf("Warning: failed to preserve local mtime for %s: %v\n", localPath, err)
		}
	}
	return written, nil
}

func statLocalPath(localPath string) (os.FileInfo, bool, error) {
	info, err := os.Stat(localPath)
	if err == nil {
		return info, true, nil
	}
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	return nil, false, fmt.Errorf("stat local path %s: %w", localPath, err)
}

func statRemotePath(sftpClient *sftp.Client, remotePath string) (os.FileInfo, bool, error) {
	info, err := sftpClient.Stat(remotePath)
	if err == nil {
		return info, true, nil
	}
	if isNotExistError(err) {
		return nil, false, nil
	}
	return nil, false, fmt.Errorf("stat remote path %s: %w", remotePath, err)
}

func detectBidirectionalKind(localInfo os.FileInfo, localExists bool, remoteInfo os.FileInfo, remoteExists bool) (string, error) {
	if localExists && remoteExists {
		if localInfo.IsDir() != remoteInfo.IsDir() {
			return "", errors.New("local and remote path types differ")
		}
		if localInfo.IsDir() {
			return "dir", nil
		}
		return "file", nil
	}
	if localExists {
		if localInfo.IsDir() {
			return "dir", nil
		}
		return "file", nil
	}
	if remoteExists {
		if remoteInfo.IsDir() {
			return "dir", nil
		}
		return "file", nil
	}
	return "", errors.New("both local and remote paths are missing")
}

func sameFileInfo(a, b os.FileInfo) bool {
	if a == nil || b == nil {
		return false
	}
	return sameFileState(fileState{Size: a.Size(), ModTime: a.ModTime()}, fileState{Size: b.Size(), ModTime: b.ModTime()})
}

func sameFileState(a, b fileState) bool {
	return a.Size == b.Size && modTimesClose(a.ModTime, b.ModTime)
}

func modTimesClose(a, b time.Time) bool {
	diff := a.Sub(b)
	if diff < 0 {
		diff = -diff
	}
	return diff <= time.Second
}

func sortedDirs(dirs map[string]struct{}) []string {
	result := make([]string, 0, len(dirs))
	for rel := range dirs {
		result = append(result, rel)
	}
	sort.Strings(result)
	return result
}

func reverseSortedDirs(dirs map[string]struct{}) []string {
	result := sortedDirs(dirs)
	sort.Slice(result, func(i, j int) bool {
		if len(result[i]) == len(result[j]) {
			return result[i] > result[j]
		}
		return len(result[i]) > len(result[j])
	})
	return result
}

func mergeDirKeys(a, b map[string]struct{}) []string {
	merged := make(map[string]struct{}, len(a)+len(b))
	for rel := range a {
		merged[rel] = struct{}{}
	}
	for rel := range b {
		merged[rel] = struct{}{}
	}
	return sortedDirs(merged)
}

func mergeFileKeys(a, b map[string]fileState) []string {
	merged := make(map[string]struct{}, len(a)+len(b))
	for rel := range a {
		merged[rel] = struct{}{}
	}
	for rel := range b {
		merged[rel] = struct{}{}
	}
	result := make([]string, 0, len(merged))
	for rel := range merged {
		result = append(result, rel)
	}
	sort.Strings(result)
	return result
}

func joinLocalRoot(root, rel string) string {
	if rel == "" {
		return root
	}
	return filepath.Join(root, filepath.FromSlash(rel))
}

func joinRemoteRoot(root, rel string) string {
	if rel == "" {
		return root
	}
	return path.Join(root, rel)
}

func resolveRemoteSyncPath(localPath, remotePath string, remoteInfo os.FileInfo, remoteExists bool) string {
	if strings.HasSuffix(remotePath, "/") || (remoteExists && remoteInfo != nil && remoteInfo.IsDir()) {
		return path.Join(remotePath, filepath.Base(localPath))
	}
	return remotePath
}

func resolveLocalSyncPath(localPath, remotePath string, localInfo os.FileInfo, localExists bool) string {
	if strings.HasSuffix(localPath, string(os.PathSeparator)) || strings.HasSuffix(localPath, "/") || strings.HasSuffix(localPath, "\\") || (localExists && localInfo != nil && localInfo.IsDir()) {
		return filepath.Join(localPath, filepath.Base(remotePath))
	}
	return localPath
}

func resolveStepTimeout(raw string, defaultTimeout time.Duration) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return defaultTimeout, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid step timeout %q: %w", raw, err)
	}
	return parsed, nil
}

func decodeStringList(values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	for _, value := range values {
		decoded, err := decodeHexIfNeeded(value)
		if err != nil {
			return nil, err
		}
		for _, item := range splitListValue(decoded) {
			item = strings.TrimSpace(item)
			if item != "" {
				result = append(result, filepath.ToSlash(item))
			}
		}
	}
	return result, nil
}

func splitListValue(value string) []string {
	if !strings.Contains(value, ",") {
		return []string{value}
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		result = append(result, part)
	}
	return result
}

func shouldSyncPath(rel string, isDir bool, includePatterns, excludePatterns []string) bool {
	rel = filepath.ToSlash(strings.TrimPrefix(rel, "./"))
	if rel == "" {
		return true
	}
	if matchesAnyPattern(rel, excludePatterns) {
		return false
	}
	if len(includePatterns) == 0 {
		return true
	}
	if matchesAnyPattern(rel, includePatterns) {
		return true
	}
	if isDir {
		prefix := rel + "/"
		for _, pattern := range includePatterns {
			if strings.HasPrefix(pattern, prefix) || strings.HasPrefix(pattern, rel+"/**") || strings.Contains(pattern, prefix) {
				return true
			}
		}
	}
	return false
}

func matchesAnyPattern(rel string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchPattern(rel, pattern) {
			return true
		}
	}
	return false
}

func matchPattern(rel, pattern string) bool {
	rel = filepath.ToSlash(rel)
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	if pattern == "" {
		return false
	}
	baseOnly := !strings.Contains(pattern, "/")
	if pattern == rel {
		return true
	}
	if !strings.ContainsAny(pattern, "*?[") {
		return path.Base(rel) == pattern || strings.HasPrefix(rel, pattern+"/")
	}
	re, err := regexp.Compile(globToRegex(pattern))
	if err != nil {
		return false
	}
	if re.MatchString(rel) {
		return true
	}
	if baseOnly {
		return re.MatchString(path.Base(rel))
	}
	return false
}

func globToRegex(pattern string) string {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		ch := pattern[i]
		switch ch {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString(".*")
				i++
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '\\':
			b.WriteByte('\\')
			b.WriteByte(ch)
		default:
			b.WriteByte(ch)
		}
	}
	b.WriteString("$")
	return b.String()
}

func addParentDirs(tree *syncTree) {
	for rel := range tree.Files {
		parent := path.Dir(rel)
		for parent != "." && parent != "/" && parent != "" {
			tree.Dirs[parent] = struct{}{}
			parent = path.Dir(parent)
		}
	}
}

func resolveRemoteUploadPath(localPath, remotePath, fileName string) string {
	destFileName := fileName
	if destFileName == "" {
		destFileName = filepath.Base(localPath)
	}
	if strings.HasSuffix(remotePath, "/") {
		return remotePath + destFileName
	}
	return remotePath
}

func resolveLocalDownloadPath(localPath, remotePath, fileName string) string {
	destFileName := fileName
	if destFileName == "" {
		destFileName = filepath.Base(remotePath)
	}
	info, err := os.Stat(localPath)
	if err == nil && info.IsDir() {
		return filepath.Join(localPath, destFileName)
	}
	if strings.HasSuffix(localPath, string(os.PathSeparator)) || strings.HasSuffix(localPath, "/") || strings.HasSuffix(localPath, "\\") {
		return filepath.Join(localPath, destFileName)
	}
	return localPath
}

func decodeHexIfNeeded(value string) (string, error) {
	if !strings.HasPrefix(value, "HEX_") {
		return value, nil
	}
	decoded, err := hex.DecodeString(strings.TrimPrefix(value, "HEX_"))
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func isNotExistError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "no such file") || strings.Contains(strings.ToLower(err.Error()), "not exist")
}
