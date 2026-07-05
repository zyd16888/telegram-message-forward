import type { GlobalThemeOverrides } from 'naive-ui'

// 保留 clayLight/clayDark 导出名，实际语义已切换为克制的中后台主题。
const fontFamily =
  '-apple-system, BlinkMacSystemFont, "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", "Segoe UI", sans-serif'

export const clayLight: GlobalThemeOverrides = {
  common: {
    fontFamily,
    fontFamilyMono: '"JetBrains Mono", "Fira Code", ui-monospace, monospace',

    primaryColor: '#2F8FD6',
    primaryColorHover: '#2584C8',
    primaryColorPressed: '#1E75B2',
    primaryColorSuppl: '#2584C8',

    successColor: '#2FB896',
    successColorHover: '#28A989',
    successColorPressed: '#228F75',
    warningColor: '#EF9A4C',
    warningColorHover: '#E18D42',
    warningColorPressed: '#C97935',
    errorColor: '#E55C54',
    errorColorHover: '#D84F48',
    errorColorPressed: '#BD413B',
    infoColor: '#2F8FD6',

    bodyColor: '#F3F7FB',
    baseColor: '#FFFFFF',
    cardColor: '#FFFFFF',
    modalColor: '#FFFFFF',
    popoverColor: '#FFFFFF',
    tableColor: '#FFFFFF',
    inputColor: '#FFFFFF',
    tagColor: '#F1F5F9',
    textColorBase: '#172033',
    textColor1: '#172033',
    textColor2: '#526071',
    textColor3: '#8A96A8',
    placeholderColor: '#9AA6B5',
    iconColor: '#64748B',
    dividerColor: '#E5E9F0',
    borderColor: '#E5E9F0',

    borderRadius: '10px',
    borderRadiusSmall: '8px',

    boxShadow1: 'none',
    boxShadow2: 'none',
    boxShadow3: 'none',
  },
  Card: {
    borderRadius: '12px',
    color: '#FFFFFF',
    colorEmbedded: '#F8FAFC',
    borderColor: '#E5E9F0',
    titleFontWeight: '700',
  },
  Button: {
    borderRadiusMedium: '10px',
    borderRadiusLarge: '12px',
    borderRadiusSmall: '9px',
    borderRadiusTiny: '8px',
    fontWeight: '600',
    heightMedium: '36px',
    heightLarge: '40px',
  },
  Input: {
    borderRadius: '10px',
    color: '#FFFFFF',
    colorFocus: '#FFFFFF',
    border: '1px solid #D8DEE8',
    borderHover: '1px solid #B7C4D5',
    borderFocus: '1px solid #2F8FD6',
    boxShadowFocus: '0 0 0 3px rgba(47, 143, 214, 0.16)',
  },
  Menu: {
    borderRadius: '10px',
    itemHeight: '42px',
    itemColorActive: 'rgba(47, 143, 214, 0.1)',
    itemColorActiveHover: 'rgba(47, 143, 214, 0.14)',
    itemTextColorActive: '#1E75B2',
    itemTextColorActiveHover: '#1E75B2',
    itemIconColorActive: '#1E75B2',
    itemIconColorActiveHover: '#1E75B2',
    itemColorHover: '#F1F5F9',
    fontSize: '14px',
  },
  Tag: {
    borderRadius: '8px',
  },
  Statistic: {
    valueFontWeight: '800',
  },
  Tabs: {
    tabBorderRadius: '14px',
  },
  Switch: {
    railColor: '#D3E0EC',
    railColorActive: '#3AA0E3',
  },
}

export const clayDark: GlobalThemeOverrides = {
  common: {
    fontFamily,
    fontFamilyMono: '"JetBrains Mono", "Fira Code", ui-monospace, monospace',

    primaryColor: '#5BB4EE',
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

    bodyColor: '#111827',
    baseColor: '#182131',
    cardColor: '#182131',
    modalColor: '#182131',
    popoverColor: '#202B3C',
    tableColor: '#182131',
    inputColor: '#111827',
    tagColor: '#202B3C',
    textColorBase: '#E5EDF7',
    textColor1: '#E5EDF7',
    textColor2: '#B6C2D2',
    textColor3: '#7D8BA0',
    placeholderColor: '#6B7E92',
    iconColor: '#8397AB',
    dividerColor: 'rgba(150, 190, 230, 0.14)',
    borderColor: 'rgba(150, 190, 230, 0.16)',

    borderRadius: '10px',
    borderRadiusSmall: '8px',

    boxShadow1: 'none',
    boxShadow2: 'none',
    boxShadow3: 'none',
  },
  Card: {
    borderRadius: '12px',
    color: '#182131',
    colorEmbedded: '#202B3C',
    borderColor: 'rgba(148, 163, 184, 0.18)',
    titleFontWeight: '700',
  },
  Button: {
    borderRadiusMedium: '10px',
    borderRadiusLarge: '12px',
    borderRadiusSmall: '9px',
    borderRadiusTiny: '8px',
    fontWeight: '600',
    heightMedium: '36px',
    heightLarge: '40px',
  },
  Input: {
    borderRadius: '10px',
    color: '#111827',
    colorFocus: '#111827',
    border: '1px solid rgba(148, 163, 184, 0.24)',
    borderHover: '1px solid rgba(148, 163, 184, 0.38)',
    borderFocus: '1px solid #5BB4EE',
    boxShadowFocus: '0 0 0 3px rgba(91, 180, 238, 0.18)',
  },
  Menu: {
    borderRadius: '10px',
    itemHeight: '42px',
    itemColorActive: 'rgba(91, 180, 238, 0.16)',
    itemColorActiveHover: 'rgba(91, 180, 238, 0.2)',
    itemTextColorActive: '#8FD0FA',
    itemTextColorActiveHover: '#8FD0FA',
    itemIconColorActive: '#8FD0FA',
    itemIconColorActiveHover: '#8FD0FA',
    itemColorHover: '#202B3C',
    fontSize: '14px',
  },
  Tag: {
    borderRadius: '8px',
  },
  Statistic: {
    valueFontWeight: '800',
  },
  Tabs: {
    tabBorderRadius: '14px',
  },
  Switch: {
    railColor: '#33475D',
    railColorActive: '#4FB0EE',
  },
}
