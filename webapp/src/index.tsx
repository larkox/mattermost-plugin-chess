import React from 'react';
import {Store, Action} from 'redux';

import {GlobalState} from 'mattermost-redux/types/store';
import {getConfig} from 'mattermost-redux/selectors/entities/general';

import manifest from './manifest';

// eslint-disable-next-line import/no-unresolved
import {PluginRegistry} from './types/mattermost-webapp';
import {challenge} from './actions';
import ChannelHeaderButton from './components/channel_header_button';

export default class Plugin {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars, @typescript-eslint/no-empty-function
    public async initialize(registry: PluginRegistry, store: Store<GlobalState, Action<Record<string, unknown>>>) {
        // @see https://developers.mattermost.com/extend/plugins/webapp/reference/
        registry.registerChannelHeaderButtonAction(
            <ChannelHeaderButton/>,
            () => {
                store.dispatch(challenge() as any);
            },
            'Chess',
            'Start a chess game.',
        );

        if (registry.registerAppBarComponent) {
            const siteUrl = getConfig(store.getState())?.SiteURL || '';
            const iconURL = `${siteUrl}/plugins/${manifest.id}/public/app-bar-icon.png`;
            registry.registerAppBarComponent(
                iconURL,
                () => {
                    store.dispatch(challenge() as any);
                },
                'Start a chess game.',
            );
        }
    }
}

declare global {
    interface Window {
        registerPlugin(id: string, plugin: Plugin): void
    }
}

window.registerPlugin(manifest.id, new Plugin());
