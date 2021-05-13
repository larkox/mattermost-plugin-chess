import {GenericAction} from 'mattermost-redux/types/actions';
import {Channel, ChannelMembership} from 'mattermost-redux/types/channels';
import React from 'react';
import {Reducer} from 'redux';

export interface PluginRegistry {
    registerPostTypeComponent(typeName: string, component: React.ElementType);
    registerPopoverUserAttributesComponent(component: React.ElementType);
    registerRightHandSidebarComponent(component: React.ElementType, name: string): RegisterRightHandSidebarComponentReturn;
    registerReducer(reducer: Reducer);
    registerChannelHeaderButtonAction(icon: React.ReactNode, action: (channel: Channel, channelMembership: ChannelMembership) => void, dropdownText: string, tooltip: string);
    registerMainMenuAction(text: React.ReactNode, action: () => void, mobileIcon: React.ReactNode);

    // Add more if needed from https://developers.mattermost.com/extend/plugins/webapp/reference
}

type RegisterRightHandSidebarComponentReturn = {
    id: string;
    showRHSPlugin: GenericAction;
    hideRHSPlugin: GenericAction;
    toggleRHSPlugin: GenericAction;
}
