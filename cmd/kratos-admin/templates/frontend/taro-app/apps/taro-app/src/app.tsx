import type { PropsWithChildren } from 'react'
import { useDidHide, useDidShow, useLaunch } from '@tarojs/taro'
import { bootstrapKratosTaroApp } from '@liujitcn/kratos-taro-app-core'
import {
  pauseNotificationPolling,
  resumeNotificationPolling,
} from '@liujitcn/kratos-taro-app-system'
import { moduleManifest } from './module-manifest'
import './app.scss'

/** Taro 宿主根组件。 */
export default function App({ children }: PropsWithChildren) {
  useLaunch(() => {
    bootstrapKratosTaroApp({ modules: moduleManifest })
  })
  useDidShow(() => resumeNotificationPolling())
  useDidHide(() => pauseNotificationPolling())
  return children
}
