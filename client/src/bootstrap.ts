import { startSessionRequest } from './startup-session'

void startSessionRequest().then(user => {
  if (user) void import('./Chat')
  else void import('./Auth')
}).catch(() => {})
void import('./main')
