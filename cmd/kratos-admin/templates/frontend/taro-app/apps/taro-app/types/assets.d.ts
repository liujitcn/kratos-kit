// Taro 构建器将 PNG 资源导入转换为可访问的图片地址。
declare module '*.png' {
  const url: string;
  export default url;
}
