<script setup lang="ts">
import { onHide, onLaunch, onShow } from '@dcloudio/uni-app'
import {
  pauseNotificationPolling,
  resumeNotificationPolling,
} from '@liujitcn/kratos-uni-app-system'

onLaunch(() => console.log('App Launch'))
onShow(() => resumeNotificationPolling())
onHide(() => pauseNotificationPolling())
</script>

<style lang="scss">
@use '@liujitcn/kratos-uni-app-core/styles/base.scss';
</style>
