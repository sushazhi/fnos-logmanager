import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import GlobalSettings from './GlobalSettings.vue';

describe('GlobalSettings', () => {
    it('should render with correct props', () => {
        const wrapper = mount(GlobalSettings, {
            props: {
                enabled: true,
                checkInterval: 30000
            }
        });

        expect(wrapper.find('h4').text()).toBe('全局设置');
        expect(wrapper.find('input[type="checkbox"]').element.checked).toBe(true);
    });

    it('should show disabled state when enabled is false', () => {
        const wrapper = mount(GlobalSettings, {
            props: {
                enabled: false,
                checkInterval: 30000
            }
        });

        expect(wrapper.find('input[type="checkbox"]').element.checked).toBe(false);
        expect(wrapper.find('select').element.disabled).toBe(true);
    });

    it('should emit update:enabled on toggle', async () => {
        const wrapper = mount(GlobalSettings, {
            props: {
                enabled: false,
                checkInterval: 30000
            }
        });

        await wrapper.find('input[type="checkbox"]').setValue(true);

        expect(wrapper.emitted('update:enabled')).toBeTruthy();
        expect(wrapper.emitted('update:enabled')?.[0]).toEqual([true]);
    });

    it('should emit update:checkInterval on select change', async () => {
        const wrapper = mount(GlobalSettings, {
            props: {
                enabled: true,
                checkInterval: 30000
            }
        });

        await wrapper.find('select').setValue(60000);

        expect(wrapper.emitted('update:checkInterval')).toBeTruthy();
        expect(wrapper.emitted('update:checkInterval')?.[0]).toEqual([60000]);
    });

    it('should render all interval options', () => {
        const wrapper = mount(GlobalSettings, {
            props: {
                enabled: true,
                checkInterval: 30000
            }
        });

        const options = wrapper.findAll('option');
        expect(options.length).toBe(4);
        expect(options[0].text()).toBe('10秒');
        expect(options[1].text()).toBe('30秒');
        expect(options[2].text()).toBe('1分钟');
        expect(options[3].text()).toBe('5分钟');
    });
});
