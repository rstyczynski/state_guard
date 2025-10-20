# Data Directory

This directory is used to store SQLite database files for the FSM system.

## Purpose

Database files are kept separate from the code to:
- Keep the project root clean
- Make it easy to backup/restore data
- Allow different databases for different environments

## Usage

### REST API

```bash
# Demo database
./bin/api --db data/demo.db

# Test database
./bin/api --db data/test.db
```

### CLI

```bash
# Demo database
./bin/fsm --db data/demo.db

# Test database
./bin/fsm --db data/test.db
```

## Database Files

Database files stored here are automatically ignored by git (see `.gitignore` in this directory).

Typical files you might see:
- `demo.db` - Demo/development database
- `test.db` - Testing database
- `*.db-shm` - SQLite shared memory files
- `*.db-wal` - SQLite write-ahead log files

## Clean Up

To remove all databases:

```bash
rm data/*.db data/*.db-shm data/*.db-wal
```
