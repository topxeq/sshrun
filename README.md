# sshrun

A simple SSH client tool implemented in Go language, capable of connecting to a specified SSH server, executing commands, transferring files, and running JSON-based deployment plans.

## Features

- Execute commands on remote SSH servers
- Upload local files to remote servers
- Download files from remote servers to local machine
- Create remote directories, remove paths, chmod paths, move files, and upload atomically
- Run multi-step deployment plans from JSON
- Sync single files or directories in push, pull, or bidirectional mode
- Filter sync targets with include/exclude glob patterns
- Support password auth and private key auth
- Optional strict host key verification via `known_hosts`
- Support connection timeout and remote command timeout
- Support hex-encoded parameters to handle special characters
- Support reading commands from files
- Cross-platform support (Windows and Linux)

## Installation

### Prerequisites

- Go 1.18 or later

### Build from source

```bash
# Windows
go build -o sshrun.exe sshrun.go

# Linux
go build -o sshrun sshrun.go
```

## Usage

All hosts, usernames, passwords, file paths, and deployment paths shown below are illustrative placeholders only. Replace them with your own environment values.

### Command Line Parameters

| Parameter | Description | Example |
|-----------|-------------|---------|
| `-host` | SSH server IP address (required) | `-host=192.168.1.1` |
| `-port` | SSH server port (default: 22) | `-port=22` |
| `-user` | SSH username (required) | `-user=root` |
| `-password` | SSH password, support HEX_ prefixed hex encoding | `-password=password` or `-password=HEX_70617373776F7264` |
| `-key` | SSH private key path | `-key=C:\Users\me\.ssh\id_rsa` |
| `-keyPassphrase` | Private key passphrase | `-keyPassphrase=secret` |
| `-knownHosts` | `known_hosts` file path | `-knownHosts=C:\Users\me\.ssh\known_hosts` |
| `-strictHostKey` | Enable strict host key verification | `-strictHostKey=true` |
| `-timeout` | SSH connection timeout | `-timeout=30s` |
| `-cmdTimeout` | Timeout for each remote command | `-cmdTimeout=60s` |
| `-cmd` | Command to execute, support HEX_ prefixed hex encoding | `-cmd=date` or `-cmd=HEX_64617465` |
| `-cmdfile` | Read commands from file, each line in the file is a command | `-cmdfile=commands.txt` |
| `-type` | Function type: `cmd`, `upload`, `download`, `mkdir`, `remove`, `chmod`, `move`, `upload_atomic`, `deploy`, `sync` | `-type=upload` |
| `-localPath` | Local file path (required for upload or download) | `-localPath=C:\files\test.txt` |
| `-remotePath` | Remote file path (required for upload or download) | `-remotePath=/home/user/files/` |
| `-fileName` | Target file name for upload or download, default value is the original file name | `-fileName=newfile.txt` |
| `-targetPath` | Target remote path for move | `-targetPath=/opt/app/current.bin` |
| `-tempPath` | Temporary remote path for `upload_atomic` | `-tempPath=/opt/app/app.bin.tmp` |
| `-mode` | Mode for chmod | `-mode=0755` |
| `-plan` | Deployment plan JSON file path | `-plan=deploy.json` |
| `-direction` | Sync direction: `push`, `pull`, `bidirectional` | `-direction=push` |
| `-recursive` | Sync directories recursively | `-recursive=true` |
| `-delete` | Delete extra files on sync target | `-delete=true` |
| `-dryRun` | Print sync actions without changing files | `-dryRun=true` |
| `-conflict` | Bidirectional conflict policy: `fail_on_conflict`, `newer_wins`, `local_wins`, `remote_wins` | `-conflict=newer_wins` |
| `-include` | Include sync glob pattern, can be repeated | `-include=dist/**` |
| `-exclude` | Exclude sync glob pattern, can be repeated | `-exclude=**/*.map` |

### Examples

#### Execute command

```bash
# Execute date command
sshrun.exe -host=192.168.1.1 -port=22 -user=root -password=password -cmd=date

# Execute multiple commands from file
sshrun.exe -host=192.168.1.1 -port=22 -user=root -password=password -cmdfile=commands.txt
```

#### Upload file

```bash
# Upload local file to remote server
sshrun.exe -host=192.168.1.1 -port=22 -user=root -password=password -type=upload -localPath=C:\files\test.txt -remotePath=/home/user/files/

# Upload file with custom name
sshrun.exe -host=192.168.1.1 -port=22 -user=root -password=password -type=upload -localPath=C:\files\test.txt -remotePath=/home/user/files/ -fileName=newfile.txt

# Upload file atomically
sshrun.exe -host=192.168.1.1 -user=root -password=password -type=upload_atomic -localPath=C:\files\app.bin -remotePath=/opt/app/app.bin -tempPath=/opt/app/app.bin.tmp
```

#### Download file

```bash
# Download file from remote server
sshrun.exe -host=192.168.1.1 -port=22 -user=root -password=password -type=download -localPath=C:\files\ -remotePath=/home/user/files/test.txt

# Download file with custom name
sshrun.exe -host=192.168.1.1 -port=22 -user=root -password=password -type=download -localPath=C:\files\ -remotePath=/home/user/files/test.txt -fileName=downloaded.txt
```

#### Remote file operations

```bash
# Create remote directory
sshrun.exe -host=192.168.1.1 -user=root -password=password -type=mkdir -remotePath=/opt/app/logs

# Remove remote path recursively
sshrun.exe -host=192.168.1.1 -user=root -password=password -type=remove -remotePath=/opt/app/old

# Change mode
sshrun.exe -host=192.168.1.1 -user=root -password=password -type=chmod -remotePath=/opt/app/app.bin -mode=0755

# Move file atomically after upload
sshrun.exe -host=192.168.1.1 -user=root -password=password -type=move -remotePath=/opt/app/app.bin.tmp -targetPath=/opt/app/app.bin
```

#### Deployment plan

```json
{
  "steps": [
    {"name": "stop old process", "type": "cmd", "cmd": "pkill -x app || true", "timeout": "10s"},
    {"name": "upload binary", "type": "upload_atomic", "local_path": "./app-linux", "remote_path": "/opt/app/app", "temp_path": "/opt/app/app.tmp"},
    {"name": "chmod binary", "type": "chmod", "remote_path": "/opt/app/app", "mode": "0755"},
    {"name": "start service", "type": "cmd", "cmd": "cd /opt/app && setsid -f ./app > logs/app.log 2>&1", "timeout": "15s"}
  ]
}
```

```bash
sshrun.exe -host=192.168.1.1 -user=root -password=password -type=deploy -plan=deploy.json
```

#### Sync files and folders

```bash
# Push local directory to remote directory
sshrun.exe -host=192.168.1.1 -user=root -password=password -type=sync -direction=push -localPath=./dist -remotePath=/opt/app/dist -recursive=true

# Pull remote logs to local directory and delete local extras
sshrun.exe -host=192.168.1.1 -user=root -password=password -type=sync -direction=pull -localPath=./logs -remotePath=/opt/app/logs -recursive=true -delete=true

# Bidirectional sync, fail if both sides changed the same file
sshrun.exe -host=192.168.1.1 -user=root -password=password -type=sync -direction=bidirectional -localPath=./configs -remotePath=/opt/app/configs -recursive=true

# Inspect changes without modifying anything
sshrun.exe -host=192.168.1.1 -user=root -password=password -type=sync -direction=push -localPath=./dist -remotePath=/opt/app/dist -recursive=true -dryRun=true

# Sync only selected files
sshrun.exe -host=192.168.1.1 -user=root -password=password -type=sync -direction=push -localPath=./dist -remotePath=/opt/app/dist -recursive=true -include=dist/** -exclude=**/*.map
```

`-delete` 目前只支持单向 `push/pull`，双向同步不会执行删除。

## Practical Notes

### Enough For Real Deployment

`sshrun` is now sufficient for common deployment workflows such as:

- Prepare remote directories
- Upload binaries atomically
- Sync config or asset directories
- Restart services with ordered commands
- Verify process state, listening ports, and logs

The recommended approach is to use `-type=deploy` with a JSON plan from the `examples/` directory and adapt it per project.

### PowerShell Invocation On Windows

When launching `sshrun.exe` from PowerShell, prefer one of these forms:

```powershell
& "D:\path\to\sshrun.exe" -host=1.2.3.4 -user=root -password=secret -cmd=date

cmd /c ""D:\path\to\sshrun.exe" -host=1.2.3.4 -user=root -password=secret -cmd=date"
```

This avoids PowerShell parsing issues when passing many `-flag=value` arguments.

### Example Plans

See the `examples/` directory for ready-to-adapt JSON plans:

- `examples/deploy-plan.json`
- `examples/sync-plan.json`

## Documentation

For more detailed documentation, please refer to [sshrun.md](sshrun.md).

## Dependencies

- `golang.org/x/crypto/ssh` - SSH client implementation
- `github.com/pkg/sftp` - SFTP client implementation

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
