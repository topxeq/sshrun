# sshrun Application Documentation

## Application Introduction

`sshrun` is a simple SSH client tool implemented in Go language, capable of connecting to a specified SSH server, executing commands, returning execution results, transferring files, executing JSON deployment plans, and synchronizing files or directories. It also supports creating directories, removing paths, changing modes, moving files, atomic uploads, and sync filtering.

## Command Line Parameters

All hosts, usernames, passwords, file paths, and deployment paths shown in this document are illustrative placeholders only. Replace them with values from your own environment.

| Parameter | Description | Example |
|-----------|-------------|---------|
| `-host` | SSH server IP address (required) | `-host=xxx` |
| `-port` | SSH server port (default: 22) | `-port=22` |
| `-user` | SSH username (required) | `-user=xxx` |
| `-password` | SSH password, support HEX_ prefixed hex encoding | `-password=xxx` or `-password=HEX_xxx` |
| `-key` | SSH private key path, support HEX_ prefixed hex encoding | `-key=C:\Users\me\.ssh\id_rsa` |
| `-keyPassphrase` | SSH private key passphrase, support HEX_ prefixed hex encoding | `-keyPassphrase=xxx` |
| `-knownHosts` | Known hosts file path, support HEX_ prefixed hex encoding | `-knownHosts=C:\Users\me\.ssh\known_hosts` |
| `-strictHostKey` | Enable strict host key verification | `-strictHostKey=true` |
| `-timeout` | SSH connection timeout | `-timeout=30s` |
| `-cmdTimeout` | Timeout for each remote command | `-cmdTimeout=60s` |
| `-cmd` | Command to execute, support HEX_ prefixed hex encoding | `-cmd=date` or `-cmd=HEX_64617465` |
| `-cmdfile` | Read commands from file, each line in the file is a command | `-cmdfile=commands.txt` |
| `-type` | Function type: `cmd`, `upload`, `download`, `mkdir`, `remove`, `chmod`, `move`, `upload_atomic`, `deploy`, `sync`, default value is `cmd` | `-type=upload` |
| `-localPath` | Local file path (required for upload or download) | `-localPath=C:\files\test.txt` |
| `-remotePath` | Remote file path (required for upload or download) | `-remotePath=/home/user/files/` |
| `-fileName` | Target file name for upload or download, default value is the original file name | `-fileName=newfile.txt` |
| `-targetPath` | Target remote path for move | `-targetPath=/opt/app/current.bin` |
| `-tempPath` | Temporary remote path for `upload_atomic` | `-tempPath=/opt/app/current.bin.tmp` |
| `-mode` | Mode for chmod | `-mode=0755` |
| `-plan` | Deployment plan JSON file path | `-plan=deploy.json` |
| `-direction` | Sync direction: `push`, `pull`, `bidirectional` | `-direction=push` |
| `-recursive` | Sync directories recursively | `-recursive=true` |
| `-delete` | Delete extra files on sync target | `-delete=true` |
| `-dryRun` | Print sync operations without changing files | `-dryRun=true` |
| `-conflict` | Conflict strategy for bidirectional sync: `fail_on_conflict`, `newer_wins`, `local_wins`, `remote_wins` | `-conflict=newer_wins` |
| `-include` | Include sync glob pattern, can be repeated | `-include=dist/**` |
| `-exclude` | Exclude sync glob pattern, can be repeated | `-exclude=**/*.map` |

## Usage Examples

### Windows System

#### 1. Direct Command Execution

```bash
# Execute date command with plain text password
sshrun.exe -host=xxx -port=22 -user=xxx -password=xxx -cmd=date

# Execute ls command with hex encoded password
sshrun.exe -host=xxx -port=22 -user=xxx -password=HEX_xxx -cmd=ls

# Execute date command with hex encoded command
sshrun.exe -host=xxx -port=22 -user=xxx -password=HEX_xxx -cmd=HEX_64617465
```

#### 2. Read Commands from File

Create command file `commands.txt`:

```
date
ls -la
```

Execute command:

```bash
sshrun.exe -host=xxx -port=22 -user=xxx -password=HEX_xxx -cmdfile=commands.txt
```

#### 3. Upload File

```bash
# Upload local file to remote server
sshrun.exe -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=upload -localPath=C:\files\test.txt -remotePath=/home/user/files/

# Upload file and specify target file name
sshrun.exe -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=upload -localPath=C:\files\test.txt -remotePath=/home/user/files/ -fileName=newfile.txt

# Upload file atomically
sshrun.exe -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=upload_atomic -localPath=C:\files\test.txt -remotePath=/opt/app/test.txt -tempPath=/opt/app/test.txt.tmp
```

#### 4. Download File

```bash
# Download file from remote server to local
sshrun.exe -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=download -localPath=C:\files\ -remotePath=/home/user/files/test.txt

# Download file and specify local file name
sshrun.exe -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=download -localPath=C:\files\ -remotePath=/home/user/files/test.txt -fileName=downloaded.txt
```

#### 5. Remote Path Operations

```bash
# Create remote directory
sshrun.exe -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=mkdir -remotePath=/opt/app/logs

# Remove remote path recursively
sshrun.exe -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=remove -remotePath=/opt/app/old

# Change mode
sshrun.exe -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=chmod -remotePath=/opt/app/app.bin -mode=0755

# Move remote file
sshrun.exe -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=move -remotePath=/opt/app/app.bin.tmp -targetPath=/opt/app/app.bin
```

#### 6. Deployment Plan

Create deployment plan file `deploy.json`:

```json
{
  "steps": [
    {"name": "prepare dirs", "type": "mkdir", "remote_path": "/opt/app/logs"},
    {"name": "stop old process", "type": "cmd", "cmd": "pkill -x app || true", "timeout": "10s"},
    {"name": "upload binary", "type": "upload_atomic", "local_path": "C:\\files\\app.bin", "remote_path": "/opt/app/app.bin", "temp_path": "/opt/app/app.bin.tmp"},
    {"name": "chmod binary", "type": "chmod", "remote_path": "/opt/app/app.bin", "mode": "0755"},
    {"name": "start app", "type": "cmd", "cmd": "cd /opt/app && setsid -f ./app.bin > logs/app.log 2>&1", "timeout": "15s"}
  ]
}
```

Execute deployment:

```bash
sshrun.exe -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=deploy -plan=deploy.json
```

#### 7. Sync Files and Directories

```bash
# Push local directory to remote directory
sshrun.exe -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=sync -direction=push -localPath=C:\files\dist -remotePath=/opt/app/dist -recursive=true

# Pull remote directory to local directory
sshrun.exe -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=sync -direction=pull -localPath=C:\files\logs -remotePath=/opt/app/logs -recursive=true -delete=true

# Simple bidirectional sync with explicit conflict strategy
sshrun.exe -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=sync -direction=bidirectional -localPath=C:\files\configs -remotePath=/opt/app/configs -recursive=true -conflict=newer_wins

# Dry run
sshrun.exe -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=sync -direction=push -localPath=C:\files\dist -remotePath=/opt/app/dist -recursive=true -dryRun=true

# Include / exclude filters
sshrun.exe -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=sync -direction=push -localPath=C:\files\dist -remotePath=/opt/app/dist -recursive=true -include=dist/** -exclude=**/*.map
```

### Linux System

#### 1. Direct Command Execution

```bash
# Execute date command with plain text password
sshrun -host=xxx -port=22 -user=xxx -password=xxx -cmd=date

# Execute ls command with hex encoded password
sshrun -host=xxx -port=22 -user=xxx -password=HEX_xxx -cmd=ls

# Execute date command with hex encoded command
sshrun -host=xxx -port=22 -user=xxx -password=HEX_xxx -cmd=HEX_64617465
```

#### 2. Read Commands from File

Create command file `commands.txt`:

```
date
ls -la
```

Execute command:

```bash
sshrun -host=xxx -port=22 -user=xxx -password=HEX_xxx -cmdfile=commands.txt
```

#### 3. Upload File

```bash
# Upload local file to remote server
sshrun -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=upload -localPath=/home/user/files/test.txt -remotePath=/home/user/files/

# Upload file and specify target file name
sshrun -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=upload -localPath=/home/user/files/test.txt -remotePath=/home/user/files/ -fileName=newfile.txt

# Upload file atomically
sshrun -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=upload_atomic -localPath=/home/user/files/test.txt -remotePath=/opt/app/test.txt -tempPath=/opt/app/test.txt.tmp
```

#### 4. Download File

```bash
# Download file from remote server to local
sshrun -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=download -localPath=/home/user/files/ -remotePath=/home/user/files/test.txt

# Download file and specify local file name
sshrun -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=download -localPath=/home/user/files/ -remotePath=/home/user/files/test.txt -fileName=downloaded.txt
```

#### 5. Deployment Plan

```bash
sshrun -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=deploy -plan=deploy.json
```

#### 6. Sync Files and Directories

```bash
sshrun -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=sync -direction=push -localPath=/home/user/dist -remotePath=/opt/app/dist -recursive=true
```

## Deployment Guidance

`sshrun` is now practical for real deployment workflows, not just ad-hoc SSH commands. A typical deployment can be expressed as:

1. Create remote directories
2. Upload binary atomically
3. Sync configs or assets
4. Restart service
5. Verify process, ports, and logs

The recommended entrypoint is `-type=deploy -plan=...` with a JSON plan.

Reference plans are included in the repository:

- `examples/deploy-plan.json`
- `examples/sync-plan.json`

## Windows PowerShell Notes

When invoking `sshrun.exe` from PowerShell, prefer `&` or `cmd /c` to avoid parsing issues:

```powershell
& "D:\path\to\sshrun.exe" -host=xxx -user=xxx -password=xxx -cmd=date

cmd /c ""D:\path\to\sshrun.exe" -host=xxx -user=xxx -password=xxx -cmd=date"
```

## Notes

1. In PowerShell, the `#` character is treated as the start of a comment, so passwords containing `#` need to use hex encoding format.
2. Each line in the command file is a command, and empty lines are ignored.
3. For commands containing special characters, it is recommended to use hex encoding format or read from a file.
4. If `-strictHostKey=true` is enabled, `-knownHosts` must also be provided.
5. For large files, `sshrun` now uses streaming transfer instead of reading the whole file into memory.
6. Directory sync requires `-recursive=true`.
7. Bidirectional sync defaults to `fail_on_conflict`; use `-conflict=` to override.
8. `-delete` currently only applies to one-way `push` or `pull` sync.
9. If a sync path needs fine-grained control, combine `-include` and `-exclude` patterns.

## Compilation Method

### Windows System

```bash
go build -o sshrun.exe sshrun.go
```

### Linux System

```bash
go build -o sshrun sshrun.go
```

## Dependencies

- `golang.org/x/crypto/ssh`：Used to implement SSH connection functionality
- `github.com/pkg/sftp`：Used to implement SFTP file transfer functionality

## Test Server

- IP address: xxx
- Username: xxx
- Password: xxx (hex encoding: HEX_xxx)
