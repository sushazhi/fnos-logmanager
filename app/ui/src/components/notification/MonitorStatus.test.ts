import { describe, it, expect, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import MonitorStatus from './MonitorStatus.vue';
import { MonitorStatusProps } from './types';

describe('MonitorStatus', () => {
    const defaultStatus: MonitorStatusProps = {
        running: false,
        watchedFiles: 10,
        activeRules: 5,
        lastCheckTime: null,
        errors: []
    };

    it('should render with correct status', () => {
        const wrapper = mount(MonitorStatus, {
            props: {
                status: defaultStatus,
                enabled: true
            }
        });

        expect(wrapper.find('h4').text()).toBe('监控状态');
        expect(wrapper.find('.status-badge').text()).toBe('已停止');
        expect(wrapper.text()).toContain('10');
        expect(wrapper.text()).toContain('5');
    });

    it('should show running status', () => {
        const wrapper = mount(MonitorStatus, {
            props: {
                status: { ...defaultStatus, running: true },
                enabled: true
            }
        });

        expect(wrapper.find('.status-badge').text()).toBe('运行中');
        expect(wrapper.find('.status-badge').classes()).toContain('running');
    });

    it('should show stopped status', () => {
        const wrapper = mount(MonitorStatus, {
            props: {
                status: defaultStatus,
                enabled: true
            }
        });

        expect(wrapper.find('.status-badge').text()).toBe('已停止');
        expect(wrapper.find('.status-badge').classes()).toContain('stopped');
    });

    it('should disable start button when running', () => {
        const wrapper = mount(MonitorStatus, {
            props: {
                status: { ...defaultStatus, running: true },
                enabled: true
            }
        });

        const buttons = wrapper.findAll('button');
        const startBtn = buttons[0];
        expect(startBtn.element.disabled).toBe(true);
    });

    it('should disable stop button when not running', () => {
        const wrapper = mount(MonitorStatus, {
            props: {
                status: defaultStatus,
                enabled: true
            }
        });

        const buttons = wrapper.findAll('button');
        const stopBtn = buttons[1];
        expect(stopBtn.element.disabled).toBe(true);
    });

    it('should disable all buttons when not enabled', () => {
        const wrapper = mount(MonitorStatus, {
            props: {
                status: defaultStatus,
                enabled: false
            }
        });

        const buttons = wrapper.findAll('button');
        for (const btn of buttons) {
            expect(btn.element.disabled).toBe(true);
        }
    });

    it('should emit start event', async () => {
        const wrapper = mount(MonitorStatus, {
            props: {
                status: defaultStatus,
                enabled: true
            }
        });

        const buttons = wrapper.findAll('button');
        await buttons[0].trigger('click');

        expect(wrapper.emitted('start')).toBeTruthy();
    });

    it('should emit stop event', async () => {
        const wrapper = mount(MonitorStatus, {
            props: {
                status: { ...defaultStatus, running: true },
                enabled: true
            }
        });

        const buttons = wrapper.findAll('button');
        await buttons[1].trigger('click');

        expect(wrapper.emitted('stop')).toBeTruthy();
    });

    it('should emit check event', async () => {
        const wrapper = mount(MonitorStatus, {
            props: {
                status: defaultStatus,
                enabled: true
            }
        });

        const buttons = wrapper.findAll('button');
        await buttons[2].trigger('click');

        expect(wrapper.emitted('check')).toBeTruthy();
    });
});
