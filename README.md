oss-diff
========

[![Circle CI](https://circleci.com/gh/caiguanhao/oss-diff.svg?style=svg)](https://circleci.com/gh/caiguanhao/oss-diff)

```
oss-diff [OPTION] LOCAL-DIR  REMOTE-DIR
                  LOCAL-FILE REMOTE-FILE

Options:
    -r, --reverse  Print LOCAL file paths to stderr, REMOTE to stdout

    -m, --md5      Verify MD5 checksum besides file name and size
    -s, --shhh     Show only file path

Status code: 0 - local and remote are identical
             1 - local has different files
             2 - remote has different files
             3 - both local and remote have different files
```

LICENSE: MIT
