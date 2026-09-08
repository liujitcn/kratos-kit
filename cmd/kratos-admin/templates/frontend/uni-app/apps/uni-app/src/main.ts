import {
  bootstrapKratosApp,
  initializeAppNavigation,
  pinia,
  registerKratosAppModules,
  registerUserStoreExtension,
} from '@liujitcn/kratos-uni-app-core'
import { createSSRApp } from 'vue'
import App from './App.vue'
import { moduleManifest } from './module-manifest'
import '@liujitcn/kratos-uni-app-system'

registerKratosAppModules(moduleManifest)
registerUserStoreExtension({
  onLogin: initializeAppNavigation,
  onLogout: initializeAppNavigation,
  onSilentLogout: initializeAppNavigation,
})

/** 创建 uni-app 宿主实例。 */
export function createApp() {
  return bootstrapKratosApp({
    app: App,
    createSSRApp,
    pinia,
    modules: moduleManifest,
  })
}
