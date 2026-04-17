#!/usr/bin/env python3
"""Helper script for remote SSH command execution."""
import pexpect
import sys

HOST = "yunxigu@192.168.2.66"
PASSWORD = "yunxigu2025"

def run(cmd, timeout=60):
    """Execute a command on the remote host via SSH."""
    child = pexpect.spawn(f'ssh -o StrictHostKeyChecking=no {HOST} "{cmd}"', timeout=timeout)
    child.expect('[Pp]assword')
    child.sendline(PASSWORD)
    child.expect(pexpect.EOF)
    output = child.before.decode('utf-8', errors='replace').strip()
    # Remove the first line (password echo/prompt residue)
    lines = output.split('\n')
    if lines and lines[0].strip() in ('', ':'):
        lines = lines[1:]
    return '\n'.join(lines)

def scp_to(local_path, remote_path, recursive=False):
    """Copy file/dir to remote host."""
    flag = "-r" if recursive else ""
    child = pexpect.spawn(f'scp {flag} -o StrictHostKeyChecking=no {local_path} {HOST}:{remote_path}', timeout=120)
    child.expect('[Pp]assword')
    child.sendline(PASSWORD)
    child.expect(pexpect.EOF)
    return child.before.decode().strip()

if __name__ == "__main__":
    if len(sys.argv) > 1:
        print(run(' '.join(sys.argv[1:])))
