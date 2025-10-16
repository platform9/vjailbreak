---
title: "User Credential Management"
description: "A guide on how to manage user credentials using vjailbreak CLI."
---

## Overview

Use `vjbctl` to manage user credentials: list users, delete users, change passwords, and refresh credentials.

## Assumptions

Before you start, ensure the following prerequisites are fulfilled:

- vJailbreak is installed and configured properly.

## Command Reference

The following commands are available for user credential management:

```bash
# List users
vjbctl user list

# create a new user
vjbctl user create <username>

# Delete a user
vjbctl user delete <username>

# Change a user's password
vjbctl user change-password <username>

# Refresh user credentials
vjbctl user refresh
```

## Usage

> Note: After any user change (e.g., password updates or deletions), run `vjbctl user refresh` for the changes to be reflected.

### List users

```bash
vjbctl user list
```

### Delete a user

Delete a user and remove their credentials from the system:

```bash
vjbctl user delete <username>
```

### Change a user's password

Change the password for an existing user:

```bash
vjbctl user change-password <username>
```

You will be prompted to enter the new password in the terminal.

### Refresh user credentials

Refresh user credentials for active users:

```bash
vjbctl user refresh
```

## Examples

The following sequence demonstrates a typical workflow for updating user credentials:

```bash
# Inspect existing users
vjbctl user list

# Create a new user
vjbctl user create john.doe
# For changes to be reflected, refresh
vjbctl user refresh

# Update password for a specific user
vjbctl user change-password jane.doe
# For changes to be reflected, refresh
vjbctl user refresh

# Remove a deprovisioned user
vjbctl user delete temp.user
# For changes to be reflected, refresh
vjbctl user refresh
```
