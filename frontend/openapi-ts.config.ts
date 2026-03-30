import { defineConfig } from '@hey-api/openapi-ts'

export default defineConfig({
  input: '../docs/contracts/openapi.yaml',
  output: 'src/api/generated',
  client: '@hey-api/client-fetch',
})
