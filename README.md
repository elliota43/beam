# beam

CLI pastebin allowing file uploads and easy url sharing.

## Basic Idea

The goal is to have it work something like this:

```
$ beam main.go

# returns: https://beam.sh/paste/x0n43hkl
```

It should also be able to upload a folder recursively and return a url with a web view
that shows the file structure as it was uploaded.


## Current Status

Currently, you can run the server via:

```bash
go run ./cmd/server
```

which will start a server on port 9001.

Then you can run the client to upload a file:

```bash
go run ./cmd/client ./README.md
```

which will upload the file and return a url like:

`http://localhost:9001/u/oDZBbI5ZGLk/README.md`


## TODO

- [ ] Recursively upload folder(s)/workspaces
- [ ] Add a web view / ui to view uploaded files
- [ ] add concurrent uploads/downloads
