# Security policy

## Reporting a vulnerability

Please **do not open a public issue** for a security problem.

Use GitHub's private reporting: on this repository, open the **Security** tab and
choose **Report a vulnerability**. Describe what you found, how to reproduce it,
and what it could do. You will get a reply as soon as the maintainer can; this is
a small project, so please allow a few days.

## What counts

Owlsort moves files on your computer, so these matter most:

- a way to make it **overwrite or delete** a file (it must never do either);
- a way to make it move a file **outside** the watched folder when it should not
  (a `..` in a destination, a symbolic link, a crafted file name);
- a way to make it **run a command** (it must not run anything the user did not write);
- anything that makes it use the **network** (it must not);
- a malformed configuration or journal that makes it act on the wrong files.

## Supported versions

Until v1.0, only the latest release is supported.

## Privacy

The journal (`journal.jsonl`) records the paths of files that were moved. It stays
on your computer and is never sent anywhere; Owlsort has no network code.
