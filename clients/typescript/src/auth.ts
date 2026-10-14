import { readFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';

function authTokenPath(): string {
  return join(homedir(), '.realmrc');
}

export function getAuthToken(): string {
  // First check environment variable for backwards compatibility
  const bearer = process.env.REALM_BEARER;
  if (bearer !== undefined) {
    return bearer;
  }

  // Then check ~/.realmrc file
  try {
    return readFileSync(authTokenPath(), 'utf8').trim();
  } catch {
    return '';
  }
}
