package main

// userSetupScript returns the root-run shell script that provisions the
// container-side user account matching the host user's UID/GID. It is
// shared between `devroom new` and `devroom enter` (first entry) so the two
// commands can't drift apart on how a room's user account is set up.
const userSetupScript = `
# Base images (e.g. ubuntu:latest) often ship a baked-in user at UID/GID
# 1000, which collides with the common default first-user UID on the host.
# If that pre-existing account isn't ours, remove it so the real user (with
# the correct DEVROOM_HOME) can be created at that UID; otherwise exec'ing
# as that UID resolves to the wrong $HOME and credential mounts are missed.
existing_user=$(getent passwd "${DEVROOM_UID}" | cut -d: -f1)
if [ -n "${existing_user}" ] && [ "${existing_user}" != "${DEVROOM_USER}" ]; then
    userdel "${existing_user}" >/dev/null 2>&1 || true
fi
getent group "${DEVROOM_GID}" >/dev/null 2>&1 || groupadd -g "${DEVROOM_GID}" "${DEVROOM_USER}"
getent passwd "${DEVROOM_UID}" >/dev/null 2>&1 || useradd -u "${DEVROOM_UID}" -g "${DEVROOM_GID}" -d "${DEVROOM_HOME}" -s /bin/bash -M "${DEVROOM_USER}"
mkdir -p "${DEVROOM_HOME}"
chown "${DEVROOM_UID}:${DEVROOM_GID}" "${DEVROOM_HOME}"
[ -f "${DEVROOM_HOME}/.bashrc" ] || { cp /etc/skel/.bashrc "${DEVROOM_HOME}/.bashrc" 2>/dev/null && chown "${DEVROOM_UID}:${DEVROOM_GID}" "${DEVROOM_HOME}/.bashrc"; } 2>/dev/null || true
[ -f "${DEVROOM_HOME}/.gitconfig" ] || cp "${DEVROOM_HOME}/.gitconfig.host-ro" "${DEVROOM_HOME}/.gitconfig" 2>/dev/null || true
chown "${DEVROOM_UID}:${DEVROOM_GID}" "${DEVROOM_HOME}/.gitconfig" 2>/dev/null || true
# Rooms have no access to the host's private signing key or an ssh-agent, and
# shouldn't be able to produce commits cryptographically signed as the real
# developer identity anyway, so strip any signing config copied from the host.
for key in commit.gpgsign tag.gpgsign gpg.format user.signingkey; do
    git config --file "${DEVROOM_HOME}/.gitconfig" --unset-all "$key" 2>/dev/null || true
done
# The account has no valid password (useradd leaves it locked), so grant
# passwordless sudo rather than one that can never actually be typed in.
usermod -aG sudo "${DEVROOM_USER}"
echo "${DEVROOM_USER} ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/devroom
chmod 0440 /etc/sudoers.d/devroom
`
