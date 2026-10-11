import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

// Testing Library only auto-cleans when test globals are enabled.
afterEach(() => {
  cleanup()
})

// jsdom has no layout, so it has no scrollIntoView. Tests spy on this stub.
Element.prototype.scrollIntoView = () => {}
