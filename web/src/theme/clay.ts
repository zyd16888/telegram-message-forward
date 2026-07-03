import type { GlobalThemeOverrides } from 'naive-ui'

/**
 * Claymorphism 主题 —— Telegram 蓝奶油配色。
 *
 * 这里只负责把「颜色 / 圆角 / 字体」交给 Naive UI 的 themeOverrides；
 * 真正让组件「鼓起来」的双阴影黏土质感放在 clay.css 里，按组件类名统一叠加，
 * 这样所有页面（包括未单独改造的）都会自动获得一致的黏土外观。
 */

// 圆角字体族：拉丁/数字走 Nunito 圆体，中文回落到系统无衬线。
const fontFamily =
  '"Nunito", "Quicksand", ui-rounded, -apple-system, "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", "Segoe UI", sans-serif'

// —— 亮色：雾蓝奶油 ——
export const clayLight: GlobalThemeOverrides = {
  common: {
    fontFamily,
    fontFamilyMono: '"JetBrains Mono", "Fira Code", ui-monospace, monospace',

    // 主色：Telegram 蓝黏土
    primaryColor: '#3AA0E3',
    primaryColorHover: '#54AEE8',
    primaryColorPressed: '#2B8FD1',
    primaryColorSuppl: '#54AEE8',

    // 点缀色：薄荷绿 / 桃 / 珊瑚
    successColor: '#3FC5A0',
    successColorHover: '#57D0B0',
    successColorPressed: '#33B492',
    warningColor: '#F5A65B',
    warningColorHover: '#F8B475',
    warningColorPressed: '#EC9A4C',
    errorColor: '#F0857D',
    errorColorHover: '#F49A93',
    errorColorPressed: '#E5726A',
    infoColor: '#3AA0E3',

    // 底色 / 文本
    bodyColor: '#EAF1F8',
    baseColor: '#FFFFFF',
    cardColor: '#F7FAFD',
    modalColor: '#F7FAFD',
    popoverColor: '#F9FBFE',
    tableColor: '#F7FAFD',
    inputColor: '#EEF4FA',
    tagColor: '#E7F0F9',
    textColorBase: '#22364A',
    textColor1: '#22364A',
    textColor2: '#3C5165',
    textColor3: '#7C8D9E',
    placeholderColor: '#9DAAB9',
    iconColor: '#7C8D9E',
    dividerColor: 'rgba(60, 110, 150, 0.12)',
    borderColor: 'rgba(60, 110, 150, 0.14)',

    // 圆角：整体偏大，黏土的柔软感来自大圆角
    borderRadius: '14px',
    borderRadiusSmall: '10px',

    // 关掉 Naive 默认的硬阴影，交给 clay.css 接管
    boxShadow1: 'none',
    boxShadow2: 'none',
    boxShadow3: 'none',
  },
  Card: {
    borderRadius: '24px',
    color: '#F7FAFD',
    colorEmbedded: '#EFF5FB',
    borderColor: 'transparent',
    titleFontWeight: '800',
  },
  Button: {
    borderRadiusMedium: '16px',
    borderRadiusLarge: '18px',
    borderRadiusSmall: '12px',
    borderRadiusTiny: '10px',
    fontWeight: '700',
    heightMedium: '38px',
    heightLarge: '44px',
  },
  Input: {
    borderRadius: '14px',
    color: '#EEF4FA',
    colorFocus: '#F4F9FD',
    border: '1px solid transparent',
    borderHover: '1px solid transparent',
    borderFocus: '1px solid #9AD0F2',
    boxShadowFocus: '0 0 0 3px rgba(58, 160, 227, 0.18)',
  },
  Menu: {
    borderRadius: '16px',
    itemHeight: '46px',
    itemColorActive: '#DCEBFA',
    itemColorActiveHover: '#D2E6F8',
    itemTextColorActive: '#1E7CC4',
    itemTextColorActiveHover: '#1E7CC4',
    itemIconColorActive: '#1E7CC4',
    itemIconColorActiveHover: '#1E7CC4',
    itemColorHover: '#E7F1FB',
    fontSize: '15px',
  },
  Tag: {
    borderRadius: '10px',
  },
  Statistic: {
    valueFontWeight: '800',
  },
  Tabs: {
    tabBorderRadius: '12px',
  },
  Switch: {
    railColor: '#D3E0EC',
    railColorActive: '#3AA0E3',
  },
}

// —— 暗色：深板岩蓝 ——
export const clayDark: GlobalThemeOverrides = {
  common: {
    fontFamily,
    fontFamilyMono: '"JetBrains Mono", "Fira Code", ui-monospace, monospace',

    primaryColor: '#4FB0EE',
    primaryColorHover: '#66BDF2',
    primaryColorPressed: '#3C9FDD',
    primaryColorSuppl: '#66BDF2',

    successColor: '#4FD3B0',
    successColorHover: '#67DCBD',
    successColorPressed: '#43C4A2',
    warningColor: '#F7B472',
    warningColorHover: '#FAC088',
    warningColorPressed: '#EFA85F',
    errorColor: '#F2938C',
    errorColorHover: '#F6A69F',
    errorColorPressed: '#E7817A',
    infoColor: '#4FB0EE',

    bodyColor: '#182432',
    baseColor: '#1E2C3C',
    cardColor: '#233448',
    modalColor: '#233448',
    popoverColor: '#283B51',
    tableColor: '#233448',
    inputColor: '#1B2938',
    tagColor: '#2A3E55',
    textColorBase: '#E4EDF6',
    textColor1: '#E4EDF6',
    textColor2: '#B9C8D8',
    textColor3: '#7F92A6',
    placeholderColor: '#6B7E92',
    iconColor: '#8397AB',
    dividerColor: 'rgba(150, 190, 230, 0.14)',
    borderColor: 'rgba(150, 190, 230, 0.16)',

    borderRadius: '14px',
    borderRadiusSmall: '10px',

    boxShadow1: 'none',
    boxShadow2: 'none',
    boxShadow3: 'none',
  },
  Card: {
    borderRadius: '24px',
    color: '#233448',
    colorEmbedded: '#1E2E40',
    borderColor: 'transparent',
    titleFontWeight: '800',
  },
  Button: {
    borderRadiusMedium: '16px',
    borderRadiusLarge: '18px',
    borderRadiusSmall: '12px',
    borderRadiusTiny: '10px',
    fontWeight: '700',
    heightMedium: '38px',
    heightLarge: '44px',
  },
  Input: {
    borderRadius: '14px',
    color: '#1B2938',
    colorFocus: '#20303F',
    border: '1px solid transparent',
    borderHover: '1px solid transparent',
    borderFocus: '1px solid #3C7AA8',
    boxShadowFocus: '0 0 0 3px rgba(79, 176, 238, 0.22)',
  },
  Menu: {
    borderRadius: '16px',
    itemHeight: '46px',
    itemColorActive: '#2C4660',
    itemColorActiveHover: '#324E6C',
    itemTextColorActive: '#7FC6F5',
    itemTextColorActiveHover: '#7FC6F5',
    itemIconColorActive: '#7FC6F5',
    itemIconColorActiveHover: '#7FC6F5',
    itemColorHover: '#26384D',
    fontSize: '15px',
  },
  Tag: {
    borderRadius: '10px',
  },
  Statistic: {
    valueFontWeight: '800',
  },
  Tabs: {
    tabBorderRadius: '12px',
  },
  Switch: {
    railColor: '#33475D',
    railColorActive: '#4FB0EE',
  },
}
