import { startServer } from './server'

/**
 * Playwright globalSetup: start the Go backend server.
 * Runs once before all test projects.
 */
export default async function globalSetup() {
  await startServer()
}
