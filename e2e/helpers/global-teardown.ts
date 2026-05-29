import { stopServer } from './server'

/**
 * Playwright globalTeardown: stop the Go backend server.
 * Runs once after all test projects complete.
 */
export default async function globalTeardown() {
  await stopServer()
}
