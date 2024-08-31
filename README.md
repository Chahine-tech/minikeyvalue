# MiniKeyValue

MiniKeyValue is a simple, lightweight key-value store written in Go. It provides a basic API for setting, getting, and deleting key-value pairs with optional expiration times.

## Features

- In-memory key-value store
- Optional expiration for keys
- Concurrency-safe operations
- Data encryption for secure storage
- Automatic cleanup of expired keys
- Persistence to disk with encrypted backups

## TODO

- [X] Implement data compression before encryption
- [X] Add version control for keys
- [X] Introduce notifications for key expirations
- [X] Support global TTL configuration
- [X] Optimize locking mechanisms
- [X] Implement lazy loading of data
- [X] Key rotation for encryption keys
- [ ] Add authentication and authorization mechanisms
- [ ] Maintain an audit log for operations
- [ ] Develop distributed support for `KeyValueStore`
- [ ] Expose a RESTful API
- [ ] Build a command-line interface (CLI)
- [ ] Create a web interface for managing keys
- [ ] Integrate monitoring and alerting tools
- [ ] Implement automated backups and restoration
- [ ] Enhance documentation with detailed guides
- [ ] Conduct performance and security testing

## Requirements

- Go 1.22.1 or higher

## Installation

To install the MiniKeyValue package, use the following command:

```bash
go get github.com/Chahine-tech/minikeyvalue
```
