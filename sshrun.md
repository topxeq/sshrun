# sshrun Application Documentation

## Application Introduction

`sshrun` is a simple SSH client tool implemented in Go language, capable of connecting to a specified SSH server, executing commands, and returning execution results. It also supports uploading local files to a specified directory and downloading files from a specified directory to the local machine.

## Command Line Parameters

| Parameter | Description | Example |
|-----------|-------------|---------|
| `-host` | SSH server IP address (required) | `-host=xxx` |
| `-port` | SSH server port (default: 22) | `-port=22` |
| `-user` | SSH username (required) | `-user=xxx` |
| `-password` | SSH password, support HEX_ prefixed hex encoding | `-password=xxx` or `-password=HEX_xxx` |
| `-cmd` | Command to execute, support HEX_ prefixed hex encoding | `-cmd=date` or `-cmd=HEX_64617465` |
| `-cmdfile` | Read commands from file, each line in the file is a command | `-cmdfile=commands.txt` |
| `-type` | Function type: cmd (execute command), upload (upload file), download (download file), default value is cmd | `-type=upload` |
| `-localPath` | Local file path (required for upload or download) | `-localPath=C:\files\test.txt` |
| `-remotePath` | Remote file path (required for upload or download) | `-remotePath=/home/user/files/` |
| `-fileName` | Target file name for upload or download, default value is the original file name | `-fileName=newfile.txt` |

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
```

#### 4. Download File

```bash
# Download file from remote server to local
sshrun.exe -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=download -localPath=C:\files\ -remotePath=/home/user/files/test.txt

# Download file and specify local file name
sshrun.exe -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=download -localPath=C:\files\ -remotePath=/home/user/files/test.txt -fileName=downloaded.txt
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
```

#### 4. Download File

```bash
# Download file from remote server to local
sshrun -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=download -localPath=/home/user/files/ -remotePath=/home/user/files/test.txt

# Download file and specify local file name
sshrun -host=xxx -port=22 -user=xxx -password=HEX_xxx -type=download -localPath=/home/user/files/ -remotePath=/home/user/files/test.txt -fileName=downloaded.txt
```

## Notes

1. In PowerShell, the `#` character is treated as the start of a comment, so passwords containing `#` need to use hex encoding format.
2. Each line in the command file is a command, and empty lines are ignored.
3. For commands containing special characters, it is recommended to use hex encoding format or read from a file.

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
