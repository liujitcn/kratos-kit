/// <reference types="vite/client" />

// 上游发布包直接导出 Vue 源文件，为生成的页面代理提供组件类型。
declare module '@liujitcn/*.vue' {
  import type { DefineComponent } from 'vue';

  const component: DefineComponent;
  export default component;
}
