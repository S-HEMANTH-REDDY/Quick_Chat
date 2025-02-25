import React from 'react';

import { Box, Heading, Center } from '@chakra-ui/react';

function Footer() {
  return (
    <Box padding={8}>
      <Center>
        <Heading size="sm">Powered by Redis and Golang</Heading>
      </Center>
      <Center>
        <Heading fontStyle={'italic'} size="sm" paddingTop={2}>
          Made by{' '}
          <a href="https://schadokar.dev" rel="noreferrer" target={'_blank'}>
            Hemanth Reddy
          </a>
        </Heading>
      </Center>
    </Box>
  );
}

export default Footer;
